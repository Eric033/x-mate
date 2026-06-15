# 用例并行执行方案

## 核心思想

**并行是 opt-in 的：只有标记了 `parallel="true"` 的 case 才有资格并行，其余一律串行。**

## 设计

### 1. 并行属性

```xml
<!-- 标记了 parallel，可以并行执行 -->
<case title="创建订单" parallel="true">
  <action>...</action>
</case>

<!-- 没有标记，一律串行 -->
<case title="系统配置检查">
  <action>...</action>
</case>
```

### 2. 执行规则

| 属性 | 行为 |
|------|------|
| 无 `parallel` 属性 | 串行：启动后阻塞，必须等它跑完才发下一个 |
| `parallel="true"` | 并行：启动 goroutine 后不阻塞，继续发下一个 |

### 3. 分发模型

**统一队列，按目录顺序分发，串行和并行交替：**

```
队列: [A(串行), B(并行), C(串行), D(并行), E(并行)]

concurrency=2 时:

分发→ A(串行): 启动, 阻塞等完成
完成→ A
分发→ B(并行): 启动 goroutine, 不阻塞
分发→ C(串行): 发现 B 还在跑, 等 B 完成
完成→ B
分发→ C(串行): 启动, 阻塞等完成
完成→ C
分发→ D(并行): 启动 goroutine, 不阻塞
分发→ E(并行): 启动 goroutine, 不阻塞
完成→ D, E
```

核心逻辑：
- 串行 case 在队列头时，必须等当前所有并行 goroutine 跑完才能启动
- 并行 case 直接 goroutine 扔出去，但并发数到了上限就暂停分发
- 队列顺序保持不变

### 4. 参数

```
--concurrency <N>   并行 case 最大并发数，默认 1（纯串行）
```

## 实现

### 改动文件

1. **`internal/runner/runner.go`** — `Run()` 方法改造
2. **`internal/runner/scheduler.go`** — 新增，并行调度逻辑
3. **`cmd/engine/main.go`** — 加 `--concurrency` 参数
4. **`internal/context/context.go`** — 加 `Concurrency` 字段
5. **`internal/config/config.go`** — 传递 concurrency 到 context

### 核心改动

```go
func (r *Runner) Run(ctx *context.TestContext) *Report {
    // 扫描所有 case
    cases := scanCases(ctx.TestBase)

    // 统一队列分发
    running := 0  // 当前并行 goroutine 数
    for _, c := range cases {
        if !c.Parallel {
            // 串行：等所有并行跑完，再执行
            r.waitAllParallel()
            result := r.runCase(ctx, c.dirName)
            report.Results = append(report.Results, result)
        } else {
            // 并行：等有空位
            for running >= ctx.Concurrency {
                result := r.waitOneParallel()
                running--
                report.Results = append(report.Results, result)
            }
            running++
            go r.runParallelCase(ctx.Clone(), c.dirName, resultChan)
        }
    }
    // 等剩余并行全部完成
    r.waitAllParallel()
}
```

### 注意事项

- 并行 case 需要**独立的 context**（变量隔离），每个 goroutine clone 一份
- report 收集通过 channel 线程安全
- 串行 case 执行时等待所有并行完成，保证顺序和变量状态一致
