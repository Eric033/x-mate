# CLAUDE.md — x-mate 项目开发约定

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

---

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

x-mate 是一个接口自动化测试框架（Go 实现），用于驱动 TCP/HTTP 接口的 XML 报文级测试。框架源自对 Apache JMeter 测试计划的重写，核心能力是加载 XML 模板 → 参数化填充 → 发送报文 → 校验响应 → 提取变量串联步骤。

## Commands

```bash
# All commands run from engine/ directory
cd engine/

# Build engine binary
go build ./cmd/engine/

# Build all packages
go build ./...

# Run all tests
go test ./...

# Run a single package's tests with verbose output
go test ./internal/runner/ -v

# Run a specific test function
go test ./internal/runner/ -run TestRunner_Run_SingleCase -v

# Run with coverage
go test ./... -cover

# Run a demo against the built-in mock server
go run ./cmd/engine/ --test-base ./sample --start-mock --profile default

# Run with verbose logging
go run ./cmd/engine/ --test-base ./sample --start-mock --verbose

# Dry-run (validate XML without executing)
go run ./cmd/engine/ --test-base ./sample --dry-run

# Run with a custom YAML profile
go run ./cmd/engine/ --test-base ./sample --profile qa

# Save report to file
go run ./cmd/engine/ --test-base ./sample --start-mock --report-file report.log

# Build the standalone mock servers
go build ./cmd/mockserver/
go build ./cmd/tcpmock/
```

## 原始框架
- 原框架基于jmeter 框架脚本 ./spec/interface.jmx
- 脚本语法（xml）格式说明 ./spec/ENGINE_SRS.md

## Architecture

### Entry Point

`engine/cmd/engine/main.go` — parse CLI flags, load YAML config, create handler registry, run tests, print report.

### Data Flow

XML template → `template.Parametrize()` injects `xpath=value` pairs → parametrized payload → network I/O (`sampler`) → response → `Verify()` checks expected values via XPath/JSONPath → `ExtractVars()` saves response fields into context for downstream steps.

### Key Packages

| Package | Role |
|---|---|
| `internal/config` | Multi-environment YAML config (Spring-style `application-{profile}.yaml`) |
| `internal/context` | Shared mutable state: variable store (mutex-protected map), service definitions, DB pool configs |
| `internal/handler` | `Handler` interface + `Registry` + `ParseStep()` XML-to-StepData deserialization |
| `internal/runner` | Test case discovery (`<testBase>/testcase/*/`), step execution loop, logging |
| `internal/sampler` | Low-level I/O: HTTP client, TCP send/recv, BCD length prefix builder, DB connection pool |
| `internal/template` | Load `<testBase>/template/template_{trancode}.xml`, parametrize via `xmlhelper.Set()` |
| `internal/vars` | Resolve `{{var}}` and `${var}` patterns from context variables |
| `internal/xmlhelper` | XPath get/set over XML using `golang.org/x/net/html` parser |
| `internal/report` | Formatted text + markdown test report output |
| `handlers/tcp` | TCP step types: `xml_set_8`, `xml_set_sas`, `xml_set`, `mca` |
| `handlers/http` | HTTP step type: `http`, `damper_set` |
| `handlers/damper` | Damper-proxied TCP step types: `tcp_damper_set`, `mca_damper_set` |
| `handlers/rsa` | RSA encryption step |
| `handlers/runtime` | Runtime expression evaluation/verification step |

### Test Case Structure

A test case is an XML file in `<testBase>/testcase/<caseDir>/`. It has three optional phases: `<setup>`, `<action>`, `<teardown>`, each containing `<step>` elements. Each step has one `<Action>` (defining the step type, server, and attrs), optional `<value>` elements (test data as xpath=value), `<Verify>` with `<result>` elements (expected values), and `<save>` with `<key>` elements (variable extraction).

See `engine/sample/testcase/case_demo/case_demo.xml` for a working example.

### Step Types

| Step Type | Handler | Protocol |
|---|---|---|
| `xml_set_8` | TCP XMLSet8Handler | TCP, BCD 8-byte prefix, 6-byte response offset |
| `xml_set_sas` | TCP XMLSetSASHandler | TCP, SAS variant |
| `xml_set` | TCP XMLSetHandler | TCP, no BCD prefix, 6-byte response offset |
| `mca` | TCP MCAHandler | TCP, CRLF appended, 8-byte response offset |
| `http` | HTTP HTTPHandler | HTTP direct |
| `damper_set` | HTTP HTTPHandler | HTTP via Damper proxy |
| `tcp_damper_set` | Damper TCPDamperSetHandler | TCP via Damper proxy |
| `mca_damper_set` | Damper MCADamperSetHandler | MCA via Damper proxy |
| `rsa` | RSA RSAHandler | In-memory encryption |
| `runtime_verify` | Runtime RuntimeVerifyHandler | Expression evaluation |

### Variable System

- `{{var}}` / `${var}` — resolved by `vars.ResolveAll()` from `context.TestContext`
- `{{random_8}}` — auto-generated per-case 8-digit random number
- `{{seq_no}}`, `{{date_str_6}}`, `{{time_str_6}}` — per-step system variables generated by `context.GenerateSystemVars()`
- Values saved via `<save>` `<key>` elements become context variables accessible in later steps
- Variable resolution happens in body text, header values, query strings, verify expected values, and SQL text

### Mock Servers

- Built-in mock (flag `--start-mock`): Goroutine serving `POST /api/order/create`, `GET /api/order/query`, `GET /health` on `:19876`
- TCP echo mock (`cmd/tcpmock`): mirrors received XML back (strips/adds BCD length prefix)
- Mock servers pair with the `MOCK` service defined in config (address `127.0.0.1:19876`)

### Config Profiles

YAML config discovery (`LoadYAML()`): `--config` path → `application.yaml` → `engine.yaml` → `config/application.yaml` → `~/.config/engine/application.yaml`. With `--profile qa`, additionally loads and merges `application-qa.yaml`. Multi-document YAML (with `---` separators) is supported. CLI flags override YAML values.
