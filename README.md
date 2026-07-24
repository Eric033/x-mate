# x-mate — 自动化接口测试引擎

x-mate 是一个 Go 实现的自动化接口测试引擎，用于驱动 TCP/HTTP 接口的 XML 报文级测试。项目源自对 Apache JMeter 测试计划的重写，提供比 JMeter 更轻量、可编程、适合 CI/CD 集成的测试解决方案。

**核心能力**：加载 XML 模板 → 参数化填充 → 发送报文 → 校验响应 → 提取变量串联步骤。

---

## 架构概览

```
CLI (main.go)
  │
  ▼
Runner (internal/runner)
  │  ├─ 扫描 testbase/testcase/* 目录
  │  ├─ 解析 flags 标签过滤器
  │  ├─ 并行/串行调度
  │  └─ 步骤执行循环
  │
  ├─► Handler 1 (TCP: xml_set / xml_set_8 / xml_set_sas / mca)
  ├─► Handler 2 (HTTP: http / damper_set)
  ├─► Handler 3 (SQL: sql_exe / sql_select / sql_update)
  ├─► Handler 4 (Damper Proxy: tcp_damper_set / mca_damper_set)
  ├─► Handler 5 (Runtime: runtime_verify)
  └─► Handler 6 (RSA: rsa)
```

| 层 | 职责 |
|---|---|
| **CLI** | 解析命令行参数、加载 YAML 环境配置、初始化上下文、创建 Handler 注册表 |
| **Runner** | 用例发现、flags 标签过滤、并行串行调度、步骤执行、结果收集 |
| **Handler** | 每种 step type 的协议交互实现（TCP/HTTP/SQL/…） |

---

## 构建

```bash
cd engine
go build -o engine ./cmd/engine
```

编译产物为 `engine` 二进制文件。

---

## 运行方式

### 基础运行

```bash
# 指定 YAML 配置，运行 core 标签的用例
engine --config qa.yaml --test-base ./sample --flags core

# 使用默认 flags（core），启动内置 mock 服务器
engine --test-base ./sample --flags core --start-mock

# 运行所有用例（忽略 flags 过滤）
engine --test-base ./sample --run-all

# 仅验证 XML 语法，不实际执行
engine --dry-run --test-base ./sample --flags core
```

### 更多参数

```bash
# 控制并行度（默认 1 = 串行）
engine --test-base ./sample --flags core --concurrency 4

# 启用详细日志
engine --test-base ./sample --flags core --start-mock --verbose

# 保存报告到文件
engine --test-base ./sample --flags core --start-mock --report-file result.log
```

---

## CLI 参数说明

| 参数 | 默认值 | 说明 |
|---|---|---|
| `--config` | `""` | YAML 环境配置文件路径 |
| `--test-base` | `""` | **（必填）** 测试用例根目录 |
| `--flags` | `"core"` | 用例标签过滤器，空格分隔多标签，大小写不敏感 |
| `--run-all` | `false` | 忽略 flags 过滤，运行所有用例 |
| `--concurrency` | `1` | 最大并行用例数（1 = 串行） |
| `--dry-run` | `false` | 仅验证 XML 格式，不执行 |
| `--verbose` | `false` | 启用详细日志 |
| `--report-file` | `""` | 将报告输出到文件（同时输出到 stdout） |
| `--start-mock` | `false` | 启动内置 HTTP Mock 服务器（:19876） |
| `--parent-guid` | `92508788-...` | 父级 GUID |

---

## YAML 环境配置

环境文件支持多环境 profile 加载（Spring 风格）。

```yaml
# config/qa.yaml
system-id: ABCDEF

environment:
  name: QA

services:
  MOCK:
    address: "127.0.0.1:19876"
  DB:
    type: sqlite3
    address: "localhost"
    database: "test.db"
```

`system-id` 是生成交易流水号的 6 位 ASCII 字母系统标识，未配置时默认为
`ZDHZDH`。`seq_no` 固定为 24 位：`system-id + YYMMDD + 12 位日内自增序号`；
序号在一次 Engine 执行内唯一，并在交易日期变化后从 `000000000001` 重新开始。

配置文件搜索路径（优先级从高到低）：
1. `--config` 参数指定路径
2. `application.yaml`
3. `engine.yaml`
4. `config/application.yaml`
5. `~/.config/engine/application.yaml`

使用 `--config qa.yaml` 加载 `qa.yaml`，同时支持多文档 YAML（`---` 分隔）。

---

## Flags 标签规则

每条用例可通过 XML 的 `flags` 属性打标签，实现按标签过滤执行。

```xml
<case flags="core" title="核心用例">  <!-- 匹配 --flags core -->
<case flags="extended" title="扩展用例">  <!-- 不匹配 --flags core -->
<case flags="core smoke" title="冒烟+核心">  <!-- 多标签 -->
<case title="无标签用例">  <!-- 无 flags → 被跳过（除非 --run-all） -->
```

**规则**：

- **多标签**：空格分隔，任意一个匹配即通过
- **大小写不敏感**：`Core` 与 `core` 视为相同
- **空 flags**：case 未设置 `flags` 属性 → **跳过**（不执行）
- **`--run-all`**：绕过所有 flags 过滤，执行所有用例

---

## 用例结构

每个用例放在独立目录下，XML 文件与目录同名：

```
<test-base>/
  testcase/
    <case_name>/
      <case_name>.xml
  template/
    template_<trancode>.xml
```

**XML 结构**：

```xml
<case flags="core" title="示例用例" parallel="true">
  <setup>
    <step desc="前置数据准备">
      <Action type="sql_update" server_index="1" />
      <value>UPDATE test_table SET status='READY' WHERE id=1</value>
    </step>
  </setup>
  <action>
    <step desc="发送交易请求">
      <Action type="xml_set" server_index="1" trancode="TRAN001" />
      <value name="//Header/TRAN_CODE">TRAN001</value>
      <value name="//Body/AMOUNT">{{random_8}}</value>
      <Verify>
        <result name="//Response/RET_CODE">000000</result>
      </Verify>
      <save>
        <key name="order_id" locator="//Response/ORDER_ID" />
      </save>
    </step>
  </action>
  <teardown>
    <step desc="清理数据">
      <Action type="sql_update" server_index="1" />
      <value>DELETE FROM orders WHERE id = {{order_id}}</value>
    </step>
  </teardown>
</case>
```

### 三个阶段

| 阶段 | 必需 | 用途 |
|---|---|---|
| `<setup>` | 否 | 前置准备（DB 初始化、数据造数） |
| `<action>` | 推荐 | 被测操作（发送请求并校验） |
| `<teardown>` | 否 | 清理数据 |

### Step 元素

| 子元素 | 说明 |
|---|---|
| `<Action>` | 定义 step type、目标服务器、接口路径/交易码等 |
| `<value>` | 参数化键值对，作为入参传给 handler |
| `<header>` | HTTP 请求头（仅 HTTP 类型） |
| `<queryString>` | HTTP 查询参数（仅 HTTP 类型） |
| `<body>` | HTTP 请求体 |
| `<Verify>` | 断言集合 |
| `<save>` | 变量提取 |

---

## Verify DSL

断言使用统一的 `<Verify><result>` 语法：

### 场景一：XML 响应（XPath 定位）

```xml
<Verify>
  <result name="//Response/RET_CODE">000000</result>
  <result name="//Response/STATUS">ACTIVE</result>
</Verify>
```

### 场景二：JSON 响应（JSONPath 定位）

```xml
<Verify>
  <result name="$.ret_code">000000</result>
  <result name="$.ret_msg">success</result>
  <result name="$.status">ACTIVE</result>
</Verify>
```

### 场景三：SQL 查询结果

```xml
<Verify>
  <result name="STATUS[0]">ACTIVE</result>
</Verify>
```

**统一规则**：引擎自动根据响应类型（XML/JSON/文本）选择合适的断言解析方式。JSON 响应优先使用 JSONPath（`$` 前缀），XML 响应使用 XPath（`//` 前缀），其余按文本匹配。

---

## 支持的 Step Type

| Step Type | 协议 | Handler | 说明 |
|---|---|---|---|
| `xml_set` | TCP | TCP XMLSetHandler | 无 BCD 前缀，6 字节响应偏移 |
| `xml_set_8` | TCP | TCP XMLSet8Handler | BCD 8 字节长度前缀，6 字节响应偏移 |
| `xml_set_sas` | TCP | TCP XMLSetSASHandler | SAS 变体 |
| `mca` | TCP | TCP MCAHandler | CRLF 追加，8 字节响应偏移 |
| `http` | HTTP | HTTP HTTPHandler | 直连 HTTP |
| `damper_set` | HTTP | HTTP HTTPHandler | 经 Damper 代理 |
| `tcp_damper_set` | TCP | Damper TCPDamperSetHandler | 经 Damper 代理的 TCP |
| `mca_damper_set` | TCP | Damper MCADamperSetHandler | 经 Damper 代理的 MCA |
| `sql_exe` | SQL | SQL SelectHandler | 执行 SQL 语句 |
| `sql_select` | SQL | SQL SelectHandler | SQL 查询并校验结果 |
| `sql_update` | SQL | SQL UpdateHandler | SQL 更新/插入/删除 |
| `runtime_verify` | Runtime | RuntimeVerifyHandler | 运行时表达式求值校验 |
| `rsa` | In-memory | RSA RSAHandler | RSA 加密/解密 |

---

## 变量系统

### 引用方式

- `{{var_name}}` / `${var_name}` — 通过 `vars.ResolveAll()` 解析
- 支持在 body、header、queryString、verify 断言、SQL 文本中使用

### 内置变量

| 变量 | 说明 |
|---|---|
| `{{random_8}}` | 每个用例生成的 8 位随机数 |
| `{{systemID}}` | YAML 配置的 6 位系统标识 |
| `{{seq_no}}` | 每步生成的 24 位日内自增流水号 |
| `{{date_str_6}}` | 每步生成的 YYMMDD 交易日期 |
| `{{time_str_6}}` | 每步生成的 YYMMDDHH 小时时间 |
| `{{case_guid}}` | 用例自动生成的 UUID |

### 变量提取

通过 `<save>` 将响应字段保存到上下文，后续步骤可直接引用：

```xml
<save>
  <key name="order_id" locator="$.order_id" />
</save>
```

后续步骤：`<body>{{order_id}}</body>`

---

## 并行执行

用例可通过 `parallel="true"` 属性标记为可并行执行：

```xml
<case flags="core" title="并行用例" parallel="true">
```

- `--concurrency N` 控制并发上限
- 串行 case 会等待所有并行 goroutine 完成后才执行
- 并行 case 自动克隆 Context 实现变量隔离

---

## 快速开始

```bash
# 1. 编译
cd engine && go build -o engine ./cmd/engine

# 2. 运行 sample（启动内置 Mock + 仅 core 标签）
./engine --test-base ./sample --flags core --start-mock

# 3. 运行全部用例
./engine --test-base ./sample --run-all --start-mock

# 4. 指定环境配置
./engine --config ./sample/config/qa.yaml --test-base ./sample --flags core --start-mock
```

Sample 目录包含：

| 路径 | 说明 |
|---|---|
| `sample/config/qa.yaml` | QA 环境配置 |
| `sample/config.yaml` | DEMO 环境配置 |
| `sample/testcase/case_demo/` | HTTP 测试示例（下单→查单→健康检查） |
| `sample/examples/case_001/` | TCP + SQL 综合示例 |
| `sample/template/` | XML 报文模板 |
