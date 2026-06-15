# CLAUDE.md — x-mate 项目开发约定

## 开发流程

- 开发任务通过 subagent 执行，subagent 使用 ds-fla 模型
- 开发代码语言默认使用 Python（engine 模块为 Go）
- 先设计后开发：设计方案输出确认后再写代码

## 架构约定

### 并行执行模型

用例通过 `<case parallel="true">` 标记并行资格，`--concurrency N` 控制并发上限。

**分发逻辑（统一队列，按目录顺序）：**
- 串行 case（无 `parallel` 属性）：等所有并行 goroutine 完成 → 阻塞执行
- 并行 case（`parallel="true"`）：获取 semaphore 槽位 → 启动 goroutine → 不阻塞继续下一个

**关键机制：**
1. **Semaphore**：带缓冲 channel `make(chan struct{}, concurrency)` 控制并发数
2. **WaitGroup**：追踪并行 goroutine，串行 case 执行前等待全部完成
3. **Context Clone**：每个并行 goroutine 深拷贝 TestContext（Variables、Services 独立）
4. **Result Channel**：并行结果异步收集，`drainParallelResults()` 批量写入 Report

## 项目结构

```
engine/
  cmd/
    engine/main.go        # CLI 入口
    mockserver/main.go     # 内嵌 mock HTTP 服务
  internal/
    config/     # YAML + CLI 配置合并
    context/    # 测试上下文（变量、服务发现）
    handler/    # 步骤处理器接口 + 注册
    runner/     # 用例执行引擎（串行 + 并行调度）
    sampler/    # TCP/HTTP/DB 连接管理
    template/   # XML 模板参数化
    vars/       # 变量提取与替换
    xmlhelper/  # XML 操作工具
    report/     # 测试报告输出
  handlers/
    http/       # HTTP 步骤处理器
    tcp/        # TCP 步骤处理器
    sql/        # SQL 步骤处理器
    damper/     # 挡板（Mock）处理器
    rsa/        # RSA 加密处理器
    runtime/    # 运行时验证处理器
  sample/       # Demo 用例 + 配置
  spec/         # 设计文档
```

## Git 规范

- Git push 网络错误自动重试 3 次，间隔 60 秒
- 模型不可用时自动切换 fallback 模型
