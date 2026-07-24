# 软件需求规格说明书 (SRS)

## Engine — 数据驱动接口自动化测试框架

| 版本 | 日期 | 作者 | 说明 |
|------|------|------|------|
| 1.0 | 2026-06-09 | - | 初始版本，基于 JMeter 原版 interface.jmx 分析编写 |

---

## 目录

1. [引言](#1-引言)
   - [1.1 编写目的](#11-编写目的)
   - [1.2 适用范围](#12-适用范围)
   - [1.3 术语与缩略语](#13-术语与缩略语)
2. [系统概述](#2-系统概述)
   - [2.1 系统目标](#21-系统目标)
   - [2.2 系统上下文](#22-系统上下文)
   - [2.3 核心设计理念](#23-核心设计理念)
3. [功能需求](#3-功能需求)
   - [3.1 命令行入口](#31-命令行入口)
   - [3.2 全局配置管理](#32-全局配置管理)
   - [3.3 测试用例组织结构](#33-测试用例组织结构)
   - [3.4 测试执行流程](#34-测试执行流程)
   - [3.5 步骤类型详解](#35-步骤类型详解)
   - [3.6 变量与模板系统](#36-变量与模板系统)
   - [3.7 预期结果验证](#37-预期结果验证)
   - [3.8 结果提取与缓存](#38-结果提取与缓存)
   - [3.9 日志与报告](#39-日志与报告)
4. [非功能需求](#4-非功能需求)
   - [4.1 性能需求](#41-性能需求)
   - [4.2 可靠性需求](#42-可靠性需求)
   - [4.3 可扩展性需求](#43-可扩展性需求)
   - [4.4 可维护性需求](#44-可维护性需求)
5. [数据定义](#5-数据定义)
   - [5.1 测试用例 XML Schema](#51-测试用例-xml-schema)
   - [5.2 模板文件 XML Schema](#52-模板文件-xml-schema)
   - [5.3 步骤输入数据格式](#53-步骤输入数据格式)
   - [5.4 预期结果数据格式](#54-预期结果数据格式)
6. [接口定义](#6-接口定义)
   - [6.1 命令行接口](#61-命令行接口)
   - [6.2 步骤处理器接口](#62-步骤处理器接口)
   - [6.3 数据库接口](#63-数据库接口)
7. [扩展性设计](#7-扩展性设计)
8. [附录](#8-附录)

---

## 1. 引言

### 1.1 编写目的

本文档旨在完整、详细地描述 Engine 接口自动化测试框架的全部功能需求，作为后续所有代码实现（Python、Go、Java 等任意语言）的权威设计依据。

Engine 框架是对原有的 Apache JMeter 测试计划 `interface.jmx` 的独立重写，去除对 JMeter 运行时环境的依赖，保留核心测试逻辑，使其成为一个独立、可编程、可扩展的接口自动化测试引擎。

### 1.2 适用范围

本文档适用于：
- 开发团队：作为功能设计和实现的唯一参照
- 测试团队：理解框架能力边界，编写测试用例
- 运维团队：部署和配置框架运行环境

### 1.3 术语与缩略语

| 术语 | 说明 |
|------|------|
| TestCase / 用例 | 一个独立的测试用例，对应一个 XML 文件 |
| Step / 步骤 | 测试用例中的最小执行单元，每个步骤包含一个 Action |
| Action | 步骤中定义具体操作的元素，含 type 属性标识步骤类型 |
| TranCode | 交易码，用于定位对应的 XML 模板文件 |
| Template / 模板 | 预定义的 XML 请求报文模板，通过参数化填充后发送 |
| Verify / 验证 | 步骤中的预期结果定义 |
| Save / 缓存 | 从响应中提取数据并存入全局变量 |
| Damper / 挡板 | Mock 服务器，用于模拟后端服务 |
| BCD | Binary-Coded Decimal，一种长度前缀编码方式 |
| XPath | XML 路径语言，用于定位 XML 文档中的节点 |
| JSONPath | JSON 路径语言，用于定位 JSON 文档中的节点 |
| JMeter Variable | `${variableName}` 格式的变量占位符 |
| Template Variable | `{{variableName}}` 格式的模板占位符 |

---

## 2. 系统概述

### 2.1 系统目标

Engine 是一个**数据驱动的接口自动化测试框架**，其核心功能是：

1. 读取指定目录下的测试用例 XML 文件
2. 解析用例中的步骤定义
3. 根据步骤类型执行相应的操作（TCP 发送、HTTP 请求、SQL 查询等）
4. 校验实际响应是否符合预期
5. 从响应中提取数据供后续步骤使用
6. 汇总生成测试报告

### 2.2 系统上下文

```
┌──────────────────────────────────────────────────────────┐
│                     Engine Framework                      │
│                                                          │
│  ┌──────────┐   ┌──────────┐   ┌────────────────────┐   │
│  │ CLI 入口  │──▶│  Runner   │──▶│  TestCase Executor │   │
│  └──────────┘   └──────────┘   └────────┬───────────┘   │
│                                         │                │
│                    ┌────────────────────┼───────┐        │
│                    ▼                    ▼       ▼        │
│             ┌──────────┐   ┌──────────┐  ┌──────────┐  │
│             │ TCP Sampler│   │HTTP Sampler│ │JDBC Sampler│ │
│             └─────┬─────┘   └─────┬─────┘  └─────┬─────┘  │
│                   ▼               ▼              ▼       │
│             ┌──────────────────────────────────────┐     │
│             │         StepResult / Assertion        │     │
│             └──────────────────────────────────────┘     │
│                                                          │
│  External Dependencies:                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │ Target   │  │ Database │  │ Damper   │               │
│  │ Server   │  │ Server   │  │ Server   │               │
│  └──────────┘  └──────────┘  └──────────┘               │
└──────────────────────────────────────────────────────────┘
```

### 2.3 核心设计理念

1. **数据驱动**：测试数据和预期结果定义在 XML 用例文件中，与执行引擎分离
2. **模板参数化**：请求报文模板通过 XPath + 测试数据填充生成实际请求
3. **变量共享上下文**：所有步骤共享一个全局变量池，实现步骤间的数据传递
4. **分派模式**：每种步骤类型对应一个独立的 Handler，通过 step_type 进行路由
5. **三段式执行**：每个用例支持 setup → action → teardown 三段生命周期

---

## 3. 功能需求

### 3.1 命令行入口

#### 3.1.1 参数定义

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `--test-base` | String | 是 | - | 测试用例根目录路径 |
| `--server` | String | 是 | - | 目标服务器地址，格式 `ip:port` 或多服务器 `ip1:port1@ip2:port2` |
| `--flags` | String | 否 | `core` | 用例标签过滤，只执行匹配标签的用例 |
| `--verbose` | Flag | 否 | - | 启用详细日志输出 |
| `--dry-run` | Flag | 否 | - | 仅验证不执行（解析 XML、校验格式） |
| `--db-info` | String | 否 | - | 数据库连接信息，格式 `ip:port:dbname:user:passwd` |
| `--damper-server` | String | 否 | - | 挡板服务器地址，格式 `ip:port:tcpPort:httpPort` |
| `--env-name` | String | 否 | `UNDEFINED` | 环境名称标识 |
| `--parent-guid` | String | 否 | 系统预置 | 父级 GUID |

#### 3.1.2 命令行示例

```bash
engine --test-base ./sample --server "10.102.55.13:9996"
engine --test-base ./sample --server "ip1:port1@ip2:port2" --flags core --verbose
engine --test-base ./sample --server "ip:port" --db-info "ip:port:name:user:passwd" \
       --damper-server "ip:port:12:45"
engine --test-base ./sample --dry-run
```

### 3.2 全局配置管理

#### 3.2.1 全局变量列表

| 变量名 | 来源 | 默认值 | 说明 |
|--------|------|--------|------|
| `testBase` | `--test-base` | - | 测试用例根目录路径 |
| `flags` | `--flags` | `core` | 用例标签过滤条件 |
| `G_server` | `--server` | `10.102.55.13:9996` | 目标服务器地址(原始格式) |
| `G_DamplerServer` | `--damper-server` | `10.102.55.13:9996:12:45` | 挡板服务器地址(原始格式) |
| `parent_guid` | `--parent-guid` | `92508788-...` | 父级 GUID |
| `DB_IP` | `--db-info` | - | 数据库 IP |
| `DB_port` | `--db-info` | - | 数据库端口 |
| `DB_name` | `--db-info` | - | 数据库名 |
| `DB_user` | `--db-info` | `readonly` | 数据库用户名 |
| `DB_passwd` | `--db-info` | `readonly` | 数据库密码 |
| `envName` | `--env-name` | `UNDEFINED` | 环境名称 |
| `systemID` | YAML `system-id` | `ZDHZDH` | 6位字母系统标识，用于生成流水号 |
| `DB_info` | `--db-info` | `UNDEFINED` | 数据库完整信息 |
| `DB_type` | `--db-info` | `UNDEFINED` | 数据库类型 |
| `tcpDamServerIP` | 从 G_DamplerServer 解析 | - | 挡板 TCP IP |
| `tcpDamServerPort` | 从 G_DamplerServer 解析 | - | 挡板 TCP 端口 |
| `httpDamServerIP` | 从 G_DamplerServer 解析 | - | 挡板 HTTP IP |
| `httpDamServerPort` | 从 G_DamplerServer 解析 | - | 挡板 HTTP 端口 |

#### 3.2.2 多服务器解析

当 `--server` 参数包含 `@` 分隔符时，表示多服务器部署。框架需要将字符串按 `@` 分割，然后每个服务器地址按 `:` 拆分为 IP 和端口。

- 例如 `"ip1:port1@ip2:port2"` → `server_1 = ip1`, `port_1 = port1`, `server_2 = ip2`, `port_2 = port2`

同样的逻辑应用于：
- `G_DamplerServer`：按 `:` 分割为 `tcpDamIP`, `tcpDamPort`, `httpDamIP`, `httpDamPort`（4 段）
- `DB_info`：按 `:` 分割为 `db_1=IP`, `db_2=port`, `db_3=dbname`, `db_4=dbtype`
- 多数据库支持：每个 DB 字段用 `@` 分隔多组配置

### 3.3 测试用例组织结构

#### 3.3.1 目录结构

```
<testBase>/
├── testcase/                    # 测试用例目录
│   ├── case_001/               # 每个用例一个子目录
│   │   └── case_001.xml        # 用例 XML 文件（文件名与目录名一致）
│   ├── case_002/
│   │   └── case_002.xml
│   └── ...
└── template/                   # 模板文件目录
    ├── template_TranCode001.xml
    ├── template_TranCode002.xml
    └── ...
```

框架启动时扫描 `<testBase>/testcase/` 下的所有子目录，每个子目录中的 XML 文件即为一个独立的测试用例。

#### 3.3.2 用例筛选

通过 `--flags` 参数过滤用例。每个用例 XML 中的 `<case>` 元素可携带标签信息，框架在加载时将用例的标签与 `flags` 参数进行匹配，只执行匹配成功的用例。

### 3.4 测试执行流程

#### 3.4.1 总体执行流程

```
1. 解析命令行参数，初始化全局配置
2. 扫描 testBase/testcase/ 目录，收集所有用例目录
3. 遍历每个用例目录：
   3.1 读取用例 XML 文件
   3.2 用 flags 过滤用例
   3.3 执行 SETUP 阶段（setup 中的步骤）
   3.4 执行 ACTION 阶段（action 中的步骤）
   3.5 执行 TEARDOWN 阶段（teardown 中的步骤）
   3.6 清理当前用例产生的临时变量
4. 汇总所有用例的执行结果，生成测试报告
```

#### 3.4.2 用例 XML 结构

每个用例 XML 文件的根元素为 `<case>`，其结构如下：

```xml
<case tittle="用例标题" title="用例标题">
  <setup>
    <step desc="准备步骤描述">
      <Action type="sql_update" ... />
      <Verify>...</Verify>
      <save>
        <key name="var1">...</key>
      </save>
    </step>
  </setup>
  <action>
    <step desc="测试步骤描述">
      <Action type="xml_set" server_index="1" trancode="TranCode001" sleep="100" />
      <value name="xpath_expr">replacement_value</value>
      <result name="xpath_expr">expected_value</result>
      <Verify>
        <result name="xpath_expr" isHeader="False" headerName="">expected_value</result>
      </Verify>
      <save>
        <key name="variableName1">//xpath/to/value</key>
        <key name="variableName2">$.jsonpath.to.value</key>
      </save>
    </step>
  </action>
  <teardown>
    <step desc="清理步骤描述">
      <Action type="sql_update" ... />
    </step>
  </teardown>
</case>
```

**关键元素说明：**
- `<case>`：根元素，`tittle` 和 `title` 属性均可作为用例标题（`title` 优先）
- `<setup>`：前置准备步骤容器
- `<action>`：核心测试步骤容器
- `<teardown>`：清理步骤容器
- `<step>`：单个执行步骤，`desc` 属性为步骤描述
- `<Action>`：步骤操作定义，包含 `type`（步骤类型）、`server_index`（目标服务器索引）、`trancode`（交易码）、`sleep`（等待毫秒数）等属性
- `<value>`：测试数据键值对，用于参数化模板
- `<result>`：预期结果键值对
- `<Verify>`：预期结果验证定义
- `<save>`：结果提取和缓存定义

#### 3.4.3 单个步骤执行流程

```
1. 解析 step XML，提取 Action 属性：
   - step_type（从 Action/type 属性获取）
   - server_index（从 Action/server_index 属性获取，默认 1）
   - trancode（从 Action/trancode 属性获取）
   - sleep（从 Action/sleep 属性获取，默认 0）
   - desc（从 step/desc 属性获取）

2. 解析测试数据：
   - 提取 <value> 子元素列表，格式为 name=value
   - 提取 <result> 子元素列表，格式为 name@@@value

3. 解析验证信息：
   - 提取 <Verify> 及其子元素

4. 解析保存键：
   - 提取 <save>/<key> 子元素列表

5. 生成系统变量：
   - seq_no：流水号 = 6位系统标识 + YYMMDD + 12位日内自增序号
   - serverIP / serverPort：根据 server_index 选择目标服务器

6. 等待 sleep 毫秒

7. 根据 step_type 路由到对应的 Handler 执行

8. Handler 返回 StepResult（包含 success/failure/error_message/extracted_vars）

9. 根据 <save>/<key> 定义，从响应中提取变量值并缓存
```

### 3.5 步骤类型详解

框架共支持 **9 大类、13 种步骤类型**：

---

#### 3.5.1 xml_set_8 — 基于 TCP 的 XML 报文交互（8 字节 BCD 长度前缀）

**概述**：通过 TCP 协议发送 XML 格式的请求报文，接收响应并进行验证。使用 8 字节 BCD 编码的长度前缀。

**Action 属性**：
| 属性 | 说明 |
|------|------|
| type | `xml_set_8` |
| server_index | 目标服务器索引（对应多服务器配置） |
| trancode | 交易码，用于定位模板文件 |
| sleep | 执行前等待毫秒数 |

**执行流程**：
1. **加载模板**：从 `<testBase>/template/template_<trancode>.xml` 读取报文模板文件
2. **收集测试数据**：从当前 step 的 `<value>` 元素提取键值对，格式 `xpath_expr:value`
3. **参数化模板**：
   - 对每对测试数据，在模板中查找对应的 XPath 表达式
   - 将找到的 XML 节点文本替换为测试数据值
   - 值可以使用 `{{variable}}` 引用上下文变量
4. **构建报文头**：
   - 将参数化后的 XML 编码为 UTF-8 字节
   - 在报文前添加 8 字节的 BCD 编码长度前缀
   - 格式：`%08d` 格式化 payload 字节长度
5. **发送 TCP 请求**：
   - 连接到 `serverIP:serverPort`
   - 发送完整报文（长度前缀 + XML 内容）
   - 超时设置：连接超时 3000ms，响应超时 60000ms
   - 报文以 EOL 字节 `0x3E` (`>`) 结尾
6. **接收响应**：
   - 接收 TCP 响应数据
   - 去除前 8 字节长度前缀，得到纯 XML 响应
7. **验证结果**：
   - 将预期结果集按 `;` 分割
   - 每个条目格式为 `xpath_expr@@@expected_value`
   - 对每个条目使用 XPath 从响应 XML 中提取实际值
   - 将实际值与预期值进行字符串比较
   - 全部匹配则通过，任一不匹配则失败
8. **缓存结果**：
   - 将去除长度前缀的响应存入 `prevResult` 变量
   - 根据 `<save>/<key>` 配置从响应中提取值并缓存

**关键差异**：使用 `BCDLengthPrefixAsciiTcpClientImpl2` 实现，长度前缀为 8 字节。

---

#### 3.5.2 xml_set_sas — 基于 TCP 的 XML 报文交互（SAS 协议变体）

**概述**：与 xml_set_8 类似，但使用不同的 TCP 实现，响应偏移量为 6 字节。

**与 xml_set_8 的区别**：
- TCP 实现类不同
- 响应数据去除前 6 字节（而非 8 字节）

**Action 属性**：与 xml_set_8 相同

**执行流程**：
1-3：与 xml_set_8 相同
4. 不添加 8 字节长度前缀（由 TCP 实现类自动处理）
5-6. 发送接收，响应的长度前缀由 TCP 实现自动处理
7. 验证时使用 `data.substring(6)` 去除前 6 字节
8. 缓存时使用 `prev.getResponseDataAsString().substring(6)` 存入 prevResult

---

#### 3.5.3 xml_set — 基于 TCP 的 XML 报文交互（标准 BCD 长度前缀）

**概述**：最常用的 TCP XML 报文交互类型，使用标准 BCD 长度前缀编码。

**与 xml_set_8 的区别**：
- 使用 `BCDLengthPrefixAsciiTcpClientImpl` 实现
- 响应偏移量为 6 字节
- 不手动添加 8 字节长度前缀（由 TCP 实现自动处理）

**Action 属性**：与 xml_set_8 相同

**执行流程**：与 xml_set_sas 基本一致，响应数据处理相同（去除前 6 字节）。

---

#### 3.5.4 mca — MCA 协议报文发送

**概述**：通过 TCP 协议发送 MCA 格式的请求报文（XML + `\r\n` 结尾），不使用长度前缀。

**Action 属性**：与 xml_set_8 相同

**执行流程**：
1-3. 加载模板、收集测试数据、参数化模板（与 xml_set 相同）
4. **构建报文**：在参数化后的 XML 末尾添加 `\r\n`
5. **发送 TCP 请求**：直接发送报文（无长度前缀），使用原始 TCP 实现
6. **接收响应**：接收原始 TCP 响应
7. **验证结果**：
   - 对每个预期结果条目使用 XPath 从响应中提取实际值
   - 响应数据使用 `data.substring(0, data.length()-2)` 去除末尾的 `\r\n`
   - 字符串比较
8. **缓存结果**：将完整响应存入 `prevResult`（不去除任何字节）

---

#### 3.5.5 sql_exe / sql_select — SQL 查询

**概述**：执行数据库 SELECT 查询语句，并对查询结果进行验证。

**step_type 值**：`sql_exe` 或 `sql_select`（两者行为完全相同）

**Action 属性**：
| 属性 | 说明 |
|------|------|
| type | `sql_exe` 或 `sql_select` |
| server_index | 数据库连接池索引（对应 pool_n） |

**SQL 语句来源**：从 `<Action>` 的子元素 `<value>` 获取 SQL 语句文本

**预期结果来源**：从 `<Verify>/<value>` 获取，同时支持从 `<result>` 元素获取（格式 `COLUMN[rowIndex]@@@expectedValue`）

**执行流程**：
1. **提取 SQL 语句**：从 `//step/Action/value` 获取 SQL 文本
2. **提取预期结果**：从 `//step/Verify/value` 获取预期结果文本
3. **解析参数占位符**：
   - 扫描 SQL 语句中的 `{{variableName}}` 模式
   - 将占位符替换为对应变量的实际值
4. **预处理 SQL**：对 SQL 语句执行变量替换（`preProcess().process(ctx, sqlstr)`）
5. **执行查询**：
   - 使用 `pool_<server_index>` 命名的数据库连接池
   - 执行 SELECT 语句
   - 结果以 `Store as String` 处理器处理
   - 每行数据存储为 Map，列名为 key
6. **验证结果**：
   - 将预期结果按 `;` 分割为多个条目
   - 每个条目格式为 `COLUMN[rowIndex]@@@expectedValue`
   - 例如 `CUSTOMER_NAME[0]@@@张三` 表示检查结果集第 0 行的 CUSTOMER_NAME 列值
   - 从 `resultVariable` 对象中按行号和列名取值
   - 字符串比较
7. **缓存结果**：
   - 将 `resultVariable` 中的查询结果用于变量提取
   - 支持通过 `sqlpath`（格式 `COLUMN[rowIndex]`）指定要提取的单元格
   - 若 sqlpath 为空，默认提取第一行第一列

---

#### 3.5.6 sql_update — SQL 更新

**概述**：执行数据库 UPDATE/INSERT/DELETE 等非查询语句。

**Action 属性**：与 sql_select 相同

**SQL 语句来源**：从 `<Action>` 的子元素 `<value>` 获取

**执行流程**：
1. **提取 SQL 语句**：从 `//step/Action/value` 获取
2. **解析参数占位符**：
   - 扫描 SQL 中的 `{{variableName}}` 占位符
   - 使用正则表达式 `\{\{(.+?)\}\}` 匹配
   - 将每个占位符替换为对应 JMeter 变量的实际值
3. **执行 SQL**：
   - 使用 `pool_<server_index>` 数据库连接池
   - 执行 `Update Statement`
   - 结果集处理器为 `Store as String`
4. **验证结果**：
   - 将预期结果（`sqlresult`）按 regex 提取修剪后的值
   - 如果 Verify 存在且预期值不为 `*`（`*` 表示跳过验证），则比对 `sqlActualResult_1` 与预期值
   - 相等则通过，不等则失败
5. **缓存结果**：
   - 默认将 `sqlActualResult_1` 的值缓存到指定变量

---

#### 3.5.7 http / damper_set — HTTP API 请求

**概述**：发送 HTTP/HTTPS 请求，支持 JSON 和 XML 响应，支持自定义 Headers 和 QueryString。

**step_type 值**：`http` 或 `damper_set`（两者执行逻辑基本相同，damper_set 使用挡板服务器地址）

**Action 属性**：
| 属性 | 说明 |
|------|------|
| type | `http` 或 `damper_set` |
| ip | HTTP 服务器 IP（可选，未设置时使用 serverIP） |
| port | HTTP 服务器端口（可选，未设置时使用 serverPort） |
| api | API 路径（支持 `{{var}}` 参数化） |
| method | HTTP 方法（GET/POST/PUT/DELETE 等） |

**子元素**：
- `<body>`：请求体内容（可选）
- `<header name="HeaderName">headerValue</header>`：自定义请求头（可多个）
- `<queryString name="paramName">paramValue</queryString>`：URL 查询参数（可多个）

**Verify 子元素格式**：
```xml
<Verify>
  <result name="xpath_or_jsonpath" isHeader="True/False" headerName="HeaderName">
    expected_value
  </result>
</Verify>
```

**执行流程**：
1. **解析 Action 属性**：提取 ip, port, api, method, body
2. **解析预期结果**：
   - 提取所有 `<result>` 元素
   - 每个 result 元素包含 4 个信息：name（定位表达式）、isHeader、headerName、value
   - 将这些信息拼接为 `name@@@isHeader@@@headerName@@@value` 的格式，用 `;` 分隔多个
3. **预处理 API 路径**：对 api 中的 `{{var}}` 占位符执行变量替换
4. **服务器路由（damper_set）**：
   - 如果 step_type 为 `damper_set`，使用挡板 HTTP 服务器地址
   - 否则使用目标服务器地址
5. **添加 QueryString**：
   - 遍历所有 `<queryString>` 元素
   - 对参数值执行变量替换
   - 若参数值包含 `#`，则设置 `alwaysEncoded = false`
   - 将参数添加到 HTTP 请求的 Arguments 中
6. **添加 Headers**：
   - 清空默认的 HeaderManager
   - 遍历所有 `<header>` 元素
   - 对 header 值执行变量替换
   - 对值进行 HTML unescape 处理
   - 将 header 添加到 HeaderManager
7. **处理 Body**：
   - 对 body 内容执行变量替换（`preProcess().process(ctx, body)`）
8. **发送 HTTP 请求**：
   - 连接 `serverIP:serverPort`
   - 请求路径 `httpserverapi`
   - HTTP 方法 `httpservermethod`
   - Content-Encoding: utf-8
   - 连接超时 3000ms，响应超时 60000ms
   - 自动跟随重定向
9. **验证结果**：
   - 将预期结果串按 `;` 分割
   - 对每个条目解析 `name@@@isHeader@@@headerName@@@expectedValue`
   - **Header 验证**（isHeader == "True"）：
     - 从响应头中查找 headerName 对应的值
     - 比对提取值与预期值
   - **Body 验证**（isHeader == "False"）：
     - 若 name 以 `$` 开头 → 使用 JSONPath 从响应 body 中提取
     - 若 name 以 `/` 开头 → 使用 XPath 从响应 body 中提取
     - 比对提取值与预期值
10. **JSONPath 断言**（damper_set 专用）：
    - 使用预期的 damper_set_results 进行 JSONPath 断言验证
11. **缓存结果**：
    - 将响应体存入 `prevResult`
    - 通过 JSONPath 从 `prevResult` 中提取值
    - 如果 jsonstr 为 `PLAIN_TEXT`，则直接缓存整个响应文本
    - 否则缓存 JSONPath 提取到的值

---

#### 3.5.8 tcp_damper_set / mca_damper_set — TCP 挡板设置

**概述**：向挡板服务器发送 TCP 报文进行挡板规则设置，用于 Mock 后端服务。

**Action 属性**：
| 属性 | 说明 |
|------|------|
| type | `tcp_damper_set` 或 `mca_damper_set` |
| trancode | 交易码，用于定位模板文件 |

**执行流程**：
1. **加载模板**：加载 `template_<trancode>.xml`
2. **收集测试数据**：提取 `<value>` 元素
3. **参数化模板**：
   - 对每个 name:value 对，先对 value 执行变量替换
   - 在模板中设置对应的 XPath 节点值
4. **修改交易码（tcp_damper_set）**：
   - 在 `//TRAN_CODE` 节点值前添加 `@` 前缀
5. **修改交易码（mca_damper_set）**：
   - 在 `//_TransactionId` 节点值前添加 `@` 前缀
6. **发送 TCP 请求**：
   - 连接到挡板 TCP 服务器 `tcpDamServerIP:tcpDamServerPort`
   - 使用 `BCDLengthPrefixAsciiTcpClientImpl` 实现
   - `reUseConnection = true`（复用连接）
   - `closeConnection = true`（发送后关闭）
7. **验证结果**：
   - tcp_damper_set: 使用 `data.substring(6)` 去除 BCD 前缀
   - mca_damper_set: 使用 `data.substring(0, data.length()-2)` 去除末尾 `\r\n`
   - 对每个预期条目使用 XPath 提取实际值
8. **缓存结果**：将处理后的响应（去除 6 字节前缀）存入 `prevResult`

---

#### 3.5.9 runtime_verify — 动态运行时验证

**概述**：不发送任何请求，仅对已有的上下文变量执行表达式求值判断。

**Action 属性**：无特殊属性

**执行流程**：
1. **获取表达式**：从 `<Verify>/<result>` 获取表达式字符串
2. **解析表达式变量**：
   - 提取表达式中的 `{{variableName}}` 占位符
   - 将每个占位符替换为对应变量的实际值
3. **执行表达式求值**：
   - 使用 Groovy 引擎对替换后的表达式求值
   - 例如 `{{var1}} > {{var2}}` → `100 > 50` → `true`
4. **判断结果**：
   - 表达式结果为 `true` → 验证通过
   - 表达式结果为 `false` → 验证失败
5. **缓存结果**（runtime save）：
   - 支持从上下文变量中提取值并缓存到新变量
   - locator 以 `//` 开头 → XPath 提取（暂未实现）
   - locator 以 `$` 开头 → JSONPath 提取
   - locator 为其他格式 → 正则表达式提取（暂未实现）

---

#### 3.5.10 rsa — RSA 加密

**概述**：使用 RSA 公钥对数据进行加密，结果存入指定变量。这是一个纯计算步骤，不发送任何网络请求。

**Action 属性**：
| 属性 | 说明 |
|------|------|
| type | `rsa` |
| key | RSA 公钥（Base64 编码） |
| value | 待加密的原始数据 |

**Save 子元素**：
- `<key>variableName</key>`：加密结果存储的变量名

**执行流程**：
1. 从 Action 的 `key` 属性获取 RSA 公钥（Base64 格式）
2. 从 Action 的 `value` 属性获取待加密数据
3. 使用 RSA/ECB/PKCS1Padding 算法加密：
   - 密钥算法：RSA
   - 最大加密块：117 字节
   - 分段加密处理
4. 将加密结果进行 Base64 编码
5. 存入 `<key>` 指定的变量名

---

### 3.5 步骤类型汇总表

| 序号 | step_type | 协议 | 请求构建 | 响应处理 | 验证方式 | 缓存方式 |
|------|-----------|------|----------|----------|----------|----------|
| 1 | `xml_set_8` | TCP | 模板参数化 + 8字节BCD长度前缀 | 去除前8字节 | XPath + @@@ 对比 | 去除前8字节后XPath提取 |
| 2 | `xml_set_sas` | TCP | 模板参数化 | 去除前6字节 | XPath + @@@ 对比 | 去除前6字节后XPath提取 |
| 3 | `xml_set` | TCP (BCD) | 模板参数化（无手动长度前缀） | 去除前6字节 | XPath + @@@ 对比 | 去除前6字节后XPath提取 |
| 4 | `mca` | TCP | 模板参数化 + `\r\n` 结尾 | 去除末尾 `\r\n` | XPath + @@@ 对比 | 完整响应 |
| 5 | `sql_exe` | JDBC | `{{}}` 变量替换 | JDBC ResultSet | COLUMN[row]@@@value | 指定行列提取 |
| 6 | `sql_select` | JDBC | `{{}}` 变量替换 + preProcess | JDBC ResultSet | COLUMN[row]@@@value | 指定行列提取 |
| 7 | `sql_update` | JDBC | `{{}}` 变量替换 | Update count | 影响行数对比 | 影响行数 |
| 8 | `http` | HTTP | Headers + QueryString + Body 参数化 | 完整HTTP响应 | Header/JSONPath/XPath对比 | JSONPath 提取 |
| 9 | `damper_set` | HTTP | 同 http + 挡板路由 | 完整HTTP响应 | JSONPath 断言 | JSONPath 提取 |
| 10 | `tcp_damper_set` | TCP (BCD) | 模板参数化 + TRAN_CODE@前缀 | 去除前6字节 | XPath + @@@ 对比 | 去除前6字节后XPath提取 |
| 11 | `mca_damper_set` | TCP | 模板参数化 + _TransactionId@前缀 | 去除末尾 `\r\n` | XPath + @@@ 对比 | 去除末尾后XPath提取 |
| 12 | `runtime_verify` | - | 无网络请求 | - | Groovy表达式求值 | JSONPath/XPath提取 |
| 13 | `rsa` | - | RSA公钥加密 | - | 无验证 | 加密结果存入变量 |

### 3.6 变量与模板系统

#### 3.6.1 变量作用域

框架维护一个全局变量表（类似 JMeter 的 `vars`），所有步骤共享这个变量表：

- **系统变量**：框架启动时自动生成（如 `seq_no`, `serverIP`, `date_str_6` 等）
- **配置变量**：从命令行参数解析得来（如 `testBase`, `G_server`, `DB_IP` 等）
- **提取变量**：步骤执行过程中从响应提取并缓存的变量
- **临时变量**：每个用例执行完毕后清理（以 `testv_` 为前缀的变量等）

#### 3.6.2 变量引用语法

| 语法 | 范围 | 说明 |
|------|------|------|
| `${variableName}` | JMeter 原生变量语法 | 直接引用变量值 |
| `{{variableName}}` | 模板变量语法 | 在 SQL 语句、HTTP body 等文本中引用变量 |

#### 3.6.3 内置系统变量

| 变量名 | 生成时机 | 值 | 说明 |
|--------|----------|-----|------|
| `date_str_6` | 步骤执行前 | 当前交易日期 YYMMDD | 6位交易日期 |
| `time_str_6` | 步骤执行前 | 当前时间 YYMMDDHH | 8位小时时间 |
| `seq_no` | 步骤执行前 | 6位系统标识 + YYMMDD + 12位日内自增序号 | 24位流水号，一次执行过程中唯一 |
| `seq_no_pay` | 步骤执行前 | 与 seq_no 相同 | 兼容旧模板的别名 |
| `time_no` | 步骤执行前 | time_str_6 | 时间编号 |
| `serverIP` | 步骤执行前 | server_<server_index> | 当前步骤的目标服务器 IP |
| `serverPort` | 步骤执行前 | port_<server_index> | 当前步骤的目标服务器端口 |
| `random_8` | 用例开始前 | 8位随机数字串 | 随机数 |
| `server_1` ~ `server_N` | 启动时 | 多服务器 IP | 目标服务器 IP 列表 |
| `port_1` ~ `port_N` | 启动时 | 多服务器端口 | 目标服务器端口列表 |
| `dbips_1` ~ `dbips_N` | 启动时 | 数据库 IP | 数据库 IP 列表 |
| `dbports_1` ~ `dbports_N` | 启动时 | 数据库端口 | 数据库端口列表 |
| `dbnames_1` ~ `dbnames_N` | 启动时 | 数据库名 | 数据库名列表 |

流水号规则：系统标识默认为 `ZDHZDH`，可通过环境 YAML 顶层
`system-id` 覆盖，且必须是 6 位 ASCII 字母。日内序号从
`000000000001` 递增至 `999999999999`，交易日期变化后从 1 重新开始；
并行用例共享同一计数器。

#### 3.6.4 变量预处理引擎 (preProcess)

框架提供一个变量预处理函数 `preProcess().process(ctx, text)`，功能如下：

1. 扫描输入文本中的 `${variableName}` 模式
2. 将匹配的变量名替换为 `vars` 中存储的实际值
3. 返回替换后的文本

该引擎主要用于：
- HTTP body 参数化
- HTTP API 路径参数化
- HTTP Headers 值参数化
- HTTP QueryString 值参数化
- SQL 语句参数化
- 测试数据值参数化

#### 3.6.5 XML 模板系统

##### 模板文件位置

`<testBase>/template/template_<trancode>.xml`

##### 模板加载

框架根据步骤中 Action 的 `trancode` 属性，拼接模板文件路径，读取文件内容为字符串。

##### 模板参数化流程

1. 解析测试数据：`name1:value1;name2:value2;...`
2. 对每对测试数据：
   - `name` 为 XPath 表达式，定位模板中需要替换的 XML 节点
   - `value` 为替换值（可包含 `{{var}}` 或 `${var}` 变量引用）
   - 对 value 执行变量预处理
   - 如果 name 不以 `/` 开头，自动添加 `//` 前缀（向前兼容）
3. 使用 XPath 在模板 XML 中查找节点，替换其文本内容
4. 返回参数化后的 XML 字符串

##### xmlhelper 工具方法

| 方法 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `set(xpath, value, xml)` | XPath表达式, 新值, XML字符串 | 修改后的XML字符串 | 在XML中查找节点并替换文本 |
| `get(xpath, xml)` | XPath表达式, XML字符串 | 节点文本值 | 从XML中提取节点文本 |

### 3.7 预期结果验证

#### 3.7.1 验证模式总览

框架支持以下验证模式：

| 验证模式 | 适用的 step_type | 断言格式 | 说明 |
|----------|-----------------|----------|------|
| XML XPath 验证 | xml_set_*, mca, tcp_*_damper_set | `xpath_expr@@@expected_value` | 从XML响应中通过XPath提取值，与预期值字符串比较 |
| SQL 结果验证 | sql_exe, sql_select | `COLUMN[rowIndex]@@@expected_value` | 从SQL查询结果中按行列定位取值比较 |
| SQL 结果验证 | sql_update | 修剪后的值与 `sqlActualResult_1` 比较 | Update 影响行数验证 |
| HTTP Body XPath 验证 | http, damper_set | `/xpath/expr@@@False@@@@@@expected` | 从HTTP响应Body中XPath取值比较 |
| HTTP Body JSONPath 验证 | http, damper_set | `$.jsonpath@@@False@@@@@@expected` | 从HTTP响应Body中JSONPath取值比较 |
| HTTP Header 验证 | http, damper_set | `name@@@True@@@HeaderName@@@expected` | 从HTTP响应Headers中取值比较 |
| 动态表达式验证 | runtime_verify | Groovy 表达式 | 对已替换变量的表达式求布尔值 |
| 跳过验证 | 所有 | 预期结果为空 或 `*` | 不执行任何验证 |

#### 3.7.2 验证结果判定规则

- **全部匹配**：所有预期结果条目都匹配 → 步骤通过
- **任一不匹配**：存在任何条目不匹配 → 步骤失败，记录不匹配的条目
- **无需验证**：预期结果为空字符串或仅包含 `*` → 跳过验证，步骤通过

#### 3.7.3 验证失败信息

验证失败时生成 `FailureMessage`：
```
[ tag xpath_expr mismatch ]
```
多个失败条目拼接：
```
[ tag xpath1 mismatch ][ tag xpath2 mismatch ]
```

### 3.8 结果提取与缓存

#### 3.8.1 Save 定义格式

在每个 `<step>` 元素中，`<save>` 子元素定义要从响应中提取的变量：

```xml
<save>
  <key name="variableName">locator_expression</key>
</save>
```

- `name` 属性：要创建的变量名
- 文本内容（locator）：定位表达式，指示从何处提取值

#### 3.8.2 不同步骤类型的缓存机制

| 步骤类型 | 提取源 | Locator 格式 | 提取方法 |
|----------|--------|-------------|----------|
| xml_set_* | `prevResult` (处理后的响应) | `//xpath/expr` | XPath 提取，值经过 preProcess |
| xml_set_sas | `prevResult` (处理后的响应) | `//xpath/expr` | XPath 提取，值经过 preProcess |
| mca | `prevResult` (完整响应) | `//xpath/expr` | XPath 提取（无 preProcess） |
| sql_select | `resultVariable` (JDBC结果集) | `COLUMN[rowIndex]` | 按行号和列名从结果集取值 |
| sql_update | `sqlActualResult_1` | （无locator） | 直接取第一行第一列的值 |
| http/damper_set | `prevResult` (响应Body) | `$.jsonpath` 或 `PLAIN_TEXT` | JSONPath 提取，PLAIN_TEXT为整个响应 |
| tcp_damper_set | `prevResult` (处理后的响应) | `//xpath/expr` | XPath 提取，值经过 preProcess |
| runtime_verify | 上下文变量 | `//xpath` 或 `$.jsonpath` 或 `regex` | JSONPath 已实现，其他预留 |

#### 3.8.3 缓存提取流程

对于使用 `<save>/<key>` 配置的步骤，框架在步骤执行完毕后执行以下流程：

1. 遍历所有 `<key>` 子元素
2. 提取 `name` 属性作为变量名
3. 提取 locator 表达式（XPath / SQL 行列 / JSONPath）
4. 根据步骤类型和 locator 格式选择提取器
5. 从目标源中提取实际值
6. 可选：对提取值执行 preProcess 变量替换
7. 将值存入全局变量表（`vars.put(keyName, value)`）

---

### 3.9 日志与报告

#### 3.9.1 日志输出

框架需要支持以下日志级别：

| 级别 | 使用场景 |
|------|----------|
| INFO | 步骤执行信息、变量值、请求/响应内容、验证结果 |
| WARN | 进入/离开每个步骤的处理阶段 |
| ERROR | 执行失败、验证失败、异常信息 |

#### 3.9.2 日志格式

- 每个步骤执行前打印分隔符和步骤描述
- 每个用例执行前打印用例标题
- 打印请求内容和响应内容
- 打印预期值和实际值对比
- 打印验证通过/失败信息
- 打印变量缓存信息

#### 3.9.3 测试报告

- 记录每个步骤的执行状态（成功/失败）
- 记录失败步骤的错误信息
- 汇总统计：总用例数、通过数、失败数、步骤通过率
- 支持将报告输出到文件

---

## 4. 非功能需求

### 4.1 性能需求

| 指标 | 要求 |
|------|------|
| 单步骤超时 | TCP 连接超时 3000ms，响应超时 60000ms |
| HTTP 超时 | 连接超时 3000ms，响应超时 60000ms |
| 数据库连接池 | 最大连接数 10，连接存活时间 5000ms，超时 10000ms |
| 步骤间等待 | 支持通过 `sleep` 属性设置等待时间（毫秒） |
| 内存占用 | 单线程执行，不应随用例数量线性增长（每用例执行后清理变量） |

### 4.2 可靠性需求

- 网络异常时需提供明确的错误信息（连接失败、超时、协议错误）
- 数据库连接失败时需提供连接信息错误提示
- XML 解析失败时需指明具体的文件路径和错误位置
- 模板文件缺失时需指明缺失的模板文件名
- 变量未定义时的占位符应当有明确的默认值

### 4.3 可扩展性需求

- 新增步骤类型：实现 Handler 接口并注册到路由器即可（见第 7 章）
- 新增协议支持：独立实现 Sampler 并在 Handler 中调用
- 新增验证模式：在 Handler 的验证阶段添加新的断言逻辑
- 新增提取模式：在缓存阶段添加新的 locator 解析器

### 4.4 可维护性需求

- 所有配置项应有明确的默认值
- 步骤处理逻辑应模块化（每种类型独立文件）
- 关键执行阶段应有日志记录
- 向前兼容：支持旧版 XML 格式中的属性名变体（如 `tittle` vs `title`）

---

## 5. 数据定义

### 5.1 测试用例 XML Schema

```xml
<?xml version="1.0" encoding="UTF-8"?>
<case tittle="用例标题" title="用例标题">
  <!-- 前置准备（可选） -->
  <setup>
    <step desc="步骤描述">
      <Action type="步骤类型" [server_index="N"] [trancode="交易码"] [sleep="毫秒"]
              [ip="IP"] [port="端口"] [api="路径"] [method="HTTP方法"]
              [key="RSA密钥"] [value="RSA原始值"] />
      <!-- 测试数据（用于模板参数化） -->
      <value name="xpath_expression">replacement_value</value>
      <!-- 预期结果（旧的格式，兼容） -->
      <result name="xpath_expression">expected_value</result>
      <!-- 验证定义 -->
      <Verify>
        <!-- HTTP result 元素 -->
        <result name="locator" isHeader="True|False" headerName="HeaderName">
          expected_value
        </result>
        <!-- SQL/动态验证的结果值 -->
        <value>expected_value</value>
      </Verify>
      <!-- 结果缓存 -->
      <save>
        <key name="variableName" [locator="xpath"] [target="targetVar"]>locator_expression</key>
      </save>
    </step>
  </setup>

  <!-- 核心测试（必需） -->
  <action>
    <step desc="步骤描述">
      <!-- 同上结构 -->
    </step>
  </action>

  <!-- 清理恢复（可选） -->
  <teardown>
    <step desc="步骤描述">
      <!-- 同上结构 -->
    </step>
  </teardown>
</case>
```

### 5.2 模板文件 XML Schema

模板文件是标准的 XML 文档，其中的叶子节点文本内容为占位符或空值，等待测试数据填充：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RootElement>
  <Header>
    <TRAN_CODE></TRAN_CODE>
    <SEQ_NO></SEQ_NO>
    <TRANS_DATE></TRANS_DATE>
  </Header>
  <Body>
    <Field1></Field1>
    <Field2></Field2>
  </Body>
</RootElement>
```

模板中每个空节点都可通过 XPath 定位并填充测试数据。

### 5.3 步骤输入数据格式

#### 5.3.1 测试数据（value 元素）

多个 `<value>` 元素定义多组参数化数据：

```xml
<value name="/RootElement/Header/TRAN_CODE">AB001</value>
<value name="/RootElement/Body/Field1">{{varName}}</value>
```

框架会将这些解析为：
```
/RootElement/Header/TRAN_CODE:AB001;/RootElement/Body/Field1:{{varName}}
```

用 `;` 分隔每组数据，每组内用 `:` 分隔 XPath 和值。

#### 5.3.2 HTTP 步骤的额外数据

| 元素 | 提取方式 | 用途 |
|------|----------|------|
| `<body>` | XPath `//body` | HTTP 请求体 |
| `<header name="N">V</header>` | Regex | 请求头 |
| `<queryString name="N">V</queryString>` | Regex | URL 查询参数 |

### 5.4 预期结果数据格式

#### 5.4.1 XML/TCP 步骤预期结果

```
xpath_expr1@@@expected_value1;xpath_expr2@@@expected_value2
```

- 每个条目用 `;` 分隔
- 条目内用 `@@@` 分隔 XPath 表达式和预期值

#### 5.4.2 SQL SELECT 步骤预期结果

```
COLUMN_NAME[0]@@@expected_value;COLUMN_NAME2[1]@@@expected_value2
```

- `COLUMN_NAME[rowIndex]` 定位结果集中的单元格
- `rowIndex` 从 0 开始

#### 5.4.3 SQL UPDATE 步骤预期结果

```
*
```
或具体的数字值（影响的行数）。`*` 表示不验证。

#### 5.4.4 HTTP 步骤预期结果

```
name1@@@False@@@@@@expected_value1;name2@@@True@@@Content-Type@@@application/json
```

- 每个条目 4 段：`name` + `isHeader` + `headerName` + `expectedValue`
- `name` 为定位表达式（XPath 或 JSONPath）
- `isHeader` = `True` → 从响应头验证，`False` → 从响应体验证
- `headerName` = 响应头名称（验证 Body 时为空）

---

## 6. 接口定义

### 6.1 命令行接口

```bash
engine [OPTIONS]

Options:
  --test-base PATH        测试用例根目录（必需）
  --server SERVER         目标服务器地址（必需）
  --flags FLAGS           用例标签过滤（默认: core）
  --verbose               启用详细日志
  --dry-run               仅验证不执行
  --db-info DB_INFO       数据库连接信息
  --damper-server DAMPER  挡板服务器地址
  --env-name ENV          环境名称（默认: UNDEFINED）
  --parent-guid GUID      父级 GUID
  --help                  显示帮助信息
  --version               显示版本信息
```

### 6.2 步骤处理器接口

每个步骤类型对应一个 Handler 类/模块，需实现以下接口：

```python
# Handler 接口（伪代码）
class StepHandler:
    """步骤处理器基类"""

    def execute(self, step_element: Element, context: TestContext) -> StepResult:
        """
        执行步骤并返回结果。

        Args:
            step_element: 步骤 XML 元素
            context: 测试上下文（包含变量表、连接池等）

        Returns:
            StepResult: 包含成功/失败状态、错误信息、提取的变量
        """
        raise NotImplementedError


class StepResult:
    success: bool              # 步骤是否成功
    failure_message: str       # 失败信息（成功时为空字符串）
    response_data: str         # 响应原始数据
    extracted_vars: dict       # 提取的变量 {name: value}
    request_data: str          # 发送的请求数据（用于日志）
```

### 6.3 数据库接口

#### 6.3.1 数据库连接池配置

框架在启动时初始化数据库连接池：

- 每个 server_index 对应一个独立的连接池，命名为 `pool_<server_index>`
- 支持多数据库：通过 `@` 分隔多组数据库配置
- 数据库驱动：Oracle（可扩展为 MySQL、PostgreSQL 等）
- 连接池参数：最大 10 连接、超时 10000ms、连接存活 5000ms、心跳 60000ms

#### 6.3.2 数据库操作

- **SELECT**：执行查询，结果以 `Map<columnName, value>` 列表形式存储
- **UPDATE/INSERT/DELETE**：执行更新，返回影响行数

---

## 7 扩展性设计

### 7.1 新增步骤类型

新增步骤类型的步骤：

1. 创建新的 Handler 类，实现 `StepHandler` 接口
2. 在 Handler 中实现：
   - 解析 Action 属性
   - 构建请求
   - 发送请求
   - 处理响应
   - 验证结果
   - 提取变量
3. 在引擎的路由表（HANDLER_MAP）中注册 step_type → Handler 映射
4. 可选用例：添加新的验证规则

### 7.2 新增协议支持

如需支持 gRPC、WebSocket、MQ 等新协议：

1. 实现对应协议的 Client/Sampler
2. 创建使用该 Client 的 Handler
3. 确保 Handler 返回统一的 StepResult

### 7.3 插件化设计

理想情况下框架应支持：
- 自定义 Handler 插件目录
- 通过配置文件注册新 Handler
- 动态加载 Handler 类

---

## 8 附录

### 8.1 完整执行流程图

```
CLI 入口
  │
  ├─ 解析命令行参数
  │   ├── --test-base → testBase
  │   ├── --server → 解析为 server_N, port_N
  │   ├── --db-info → 解析为 DB_IP, DB_port, DB_name, DB_user, DB_passwd
  │   ├── --damper-server → 解析为 damper IP/Port
  │   └── --flags → flags 过滤条件
  │
  ├─ 初始化数据库连接池
  │   └── pool_1, pool_2, pool_3 (JDBC DataSource)
  │
  ├─ 扫描测试用例目录
  │   └── ${testBase}/testcase/*/ → 用例列表
  │
  └─ 遍历每个用例目录
      │
      ├─ 读取用例 XML
      │   └── ${testBase}/testcase/${dirName}/→ case XML
      │
      ├─ 用例标签过滤
      │   ├── 匹配 → 继续
      │   └── 不匹配 → 跳过
      │
      ├─ [SETUP]
      │   └─ 遍历 setup 中的每个 step
      │       ├── 解析 Action/type → step_type
      │       ├── 路由到对应 Handler
      │       └── 执行 Handler.execute()
      │
      ├─ [ACTION]
      │   └─ 遍历 action 中的每个 step
      │       ├── 生成 seq_no / serverIP 等系统变量
      │       ├── 解析 Action 属性
      │       ├── 解析测试数据 (value 元素)
      │       ├── 解析预期结果 (result 元素 / Verify 元素)
      │       ├── 解析保存键 (save/key 元素)
      │       ├── sleep 等待
      │       ├── 路由到对应 Handler
      │       ├── Handler.execute():
      │       │   ├── 构建请求 (模板参数化、变量替换)
      │       │   ├── 发送请求 (TCP/HTTP/JDBC)
      │       │   ├── 处理响应 (去除前缀、编码转换)
      │       │   └── 验证结果 (XPath/JSONPath/表达式)
      │       └── 提取并缓存变量
      │
      ├─ [TEARDOWN]
      │   └─ 遍历 teardown 中的每个 step
      │       └── 同上
      │
      └─ 清理临时变量
          ├── 清理 testv_* 变量
          ├── 清理 setup_1, teardown_1
          └── 清理 server_index, resultVariable
```

### 8.2 已知限制（原 JMeter 版本）

1. 单线程串行执行，不支持并发
2. runtime_verify 的 XPath 提取和正则提取未实现
3. HTTP 宏变量替换（body 中的 `{{var}}`）功能被禁用
4. 数据库仅支持 Oracle
5. TCP damper 连接不复用（每次发送后关闭）

### 8.3 向后兼容性说明

以下语法差异已做兼容处理：
- `<case>` 的 `tittle` 与 `title` 属性（优先 `title`）
- XPath 表达式中 `//` 前缀自动补全
- 旧的 `tittle` 拼写错误保留支持
- `<result>` 与 `<Verify>/<result>` 双格式支持

### 8.4 参考

- 原始 JMeter 测试计划：`interface.jmx`
- JMeter 版本：4.0 (r1823414)
- JDBC 驱动：Oracle JDBC Driver
- 自定义 Java 包：`test.xmlhelper`, `test.preProcess`, `test.jsonhelper`, `test.FindByConf`
