# 旧版 XML 用例迁移工具 — 软件需求规格说明

> 版本: 1.0
>
> 作者: x-mate 团队
>
> 日期: 2026-06-15

---

## 1. 设计决策

### 1.1 方案选择：离线迁移工具 vs 运行时 LegacyLoader

| 维度 | 离线迁移工具 (Python/Go) | 运行时 LegacyLoader (Go Engine) |
|------|--------------------------|-------------------------------|
| 架构复杂度 | 低。一次性转换，输出直接能被新引擎消费 | 高。引擎需要同时理解两套 XML schema |
| 维护成本 | 低。迁移工具完成使命后归档 | 高。两个 parser 共存的 bug 空间成倍增加 |
| 错误诊断 | 好。转换时即可报告所有问题（不支持的语法、格式错误） | 差。运行时才能发现问题，用户需要 debug step |
| 迁移质量 | 高。转换后可做 dry-run 验证，确定性输出 | 中。运行时兼容需要处理大量边界情况 |
| 对引擎影响 | 零。引擎只认新 DSL | 大。引擎代码中充斥 `if old semantic` 判断 |
| 渐进迁移 | 好。按项目/目录分批次转换 | 好。但旧文件永远需要 LegacyLoader |
| 结果可审计 | 好。迁移报告列出每个 case 的转换状态 | 差。转换在运行中无痕完成 |
| 执行性能 | N/A（一次性成本） | 差。每次运行解析适配增加开销 |
| 测试验证 | 好。迁移后可用原 JMeter 和新引擎双跑验证 | 困难。运行时无法对比原始执行结果 |

**结论：采用离线迁移工具。**

理由：
1. 旧 XML 结构（`<root>` 包装、一文件多 case、`<preActions>`、`<value>` 嵌入 `<Action>` 内部、save key/locator 方向反转等）与新 DSL 差异巨大，运行时兼容引入的复杂性不值得。
2. 离线迁移后，新引擎完全不需要理解旧格式，保持纯净。
3. 通过迁移报告可以清晰地追踪 615 个文件、1010 个 case 的迁移状态，保证没有静默丢失。
4. 对于不支持的旧动作（如 `if`、`test`），可以输出警告而不是静默删除。

### 1.2 语言选择

**推荐 Go 语言**（与引擎同语言，共享 model 类型和 XML helper）：

```go
// 可直接复用 engine 包中的 XML 类型定义
import "x-mate/engine/internal/handler"
import "x-mate/engine/internal/xmlhelper"
```

备选 Python：开发速度快，但需要重新实现 XML 操作逻辑。

### 1.3 文件组织

迁移工具输出目录与引擎输入目录结构兼容：

```
output/
└── testcase/
    ├── 010101_001_活期存入功能测试_成功/
    │   └── 010101_001_活期存入功能测试_成功.xml
    ├── 010101_002_活期存入功能测试_失败/
    │   └── 010101_002_活期存入功能测试_失败.xml
    └── ...
```

---

## 2. 旧 DSL → 新 DSL 字段映射表

> 说明：旧 DSL 指存储在 `interface.jmx` 驱动下的 XML 测试资产格式。
> 新 DSL 指 `ENGINE_SRS.md` 中定义的 `<case>` 根元素格式。
> 旧 `save` 语义：`<key name="XPath_or_Locator">variableName</key>`（name=定位，内容=变量名）
> 新 `save` 语义：`<key name="variableName" locator="XPath">` 或 `<key name="variableName">XPath_or_JSONPath</key>`（推荐）
> 新 DSL 的 `<save><key>` 中有两种模式: content 文本 → old content→locator; locator attr → new locator attr

### 2.1 顶层结构

| 旧 DSL 结构 | 新 DSL 结构 | 映射方式 |
|------------|------------|---------|
| `<root>` 包裹整个文件 | 无包裹，每个文件一个 `<case>` | 剥离 `<root>`，保留内部 `<case>` 内容 |
| 单个 XML 文件包含多个 `<case>` | 每个 `<case>` 拆为独立文件 | 按 `<case>` 分割，文件名由 case 信息自动生成 |
| `<preActions>` (声明式设置) | 迁移到 `<setup>` | 将每个 `xml_setting` 转为 `<setup>` 中的 `runtime_verify` 步骤 + 上下文变量设置 |
| `<preAction>` (注意单数，也用于包裹 case) | 忽略，作为 `<root>` 同质处理 | 同 `<root>` 处理逻辑 |
| `flag` 属性（单数） | `flags` 属性（复数） | 重命名属性，值不变（多个标签空格分隔） |
| `id` 属性 | 忽略，由 `guid` 替代 | 删除旧 `id`，迁移工具自动生成本地 `guid` |
| `tittle` 属性（注意拼写错误） | `title` 属性 | 映射到 `title`，保留原始值 |
| `trancode` 属性（在 `<case>` 级别） | `trancode` 属性（在 `<Action>` 级别） | 从 `<case>` 下移至每个 `<step>` 内的 `<Action>` 中 |
| 用例 `id`（`id="0001"`, `id="0002"`） | 文件名区分 | 用 `id` 值生成文件名后缀 `_001`、`_002` |

### 2.2 步骤结构与 Action 属性

| 旧 DSL 结构 | 新 DSL 结构 | 映射方式 |
|------------|------------|---------|
| `<step desc="...">` → 直接包含 `<Action>` | `<step desc="...">` → 包含 `<Action>` | 基本不变，注意子元素层级调整 |
| `<Action type="xml_set">` | `<Action type="xml_set">` | 直接保留 |
| `<Action type="xml_set_8">` | `<Action type="xml_set_8">` | 直接保留 |
| `<Action type="xml_set_sas">` | 映射为 `xml_set_sas` | 类型名称保持不变 |
| `<Action type="mca">` | `<Action type="mca">` | 直接保留 |
| `<Action type="sql_exe">` | `<Action type="sql_exe">` | 直接保留 |
| `<Action type="sql_select">` | `<Action type="sql_select">` | 直接保留 |
| `<Action type="sql_update">` | `<Action type="sql_update">` | 直接保留 |
| `<Action type="damper_set">` | `<Action type="damper_set">` | 直接保留 |
| `<Action type="http">` | `<Action type="http">` | 直接保留 |
| `<Action type="test">` | **不支持** | 删除，迁移报告标记 WARNING |
| `<Action type="if">` | **不支持** | 删除，迁移报告标记 WARNING |

### 2.3 `<value>` 元素位置

| 旧 DSL | 新 DSL | 映射方式 |
|--------|--------|---------|
| `<value name="XPath">value</value>` 位于 `<Action>` **内部/子级** | `<value name="XPath">value</value>` 位于 `<step>` 的直接子级 | **上移一层**：从 `<Action>` 的子级提升到 `<step>` 的子级 |
| `<value>` 在 `<Action>` 内部作为 chardata（SQL 语句） | `<value>` 在 `<Action>` 外部 → `<step>` 下的 `<value>` | 对于 SQL 类型，从 Action 的 chardata 或 Action 内部的 value 提出到 `<step>` 的直接子级 `<value>` |
| `<value name="XPath">` 使用裸标签名（如 `BASE_ACCT_NO`） | `<value name="XPath">` 可以带或不带 `//` | **补全 XPath**：对裸标签名加 `//` 前缀，即 `BASE_ACCT_NO` → `//BASE_ACCT_NO`（旧 JMeter 实现自动加的） |

### 2.4 `<Verify>` / `<result>` 元素

| 旧 DSL | 新 DSL | 映射方式 |
|--------|--------|---------|
| `<result name="RET_CODE">000000</result>` 位于 `<step>` 直接子级（与 Action 并列） | `<result name="//RET_CODE">000000</result>` 位于 `<Verify>` 内部 | 包裹到 `<Verify>` 中；对裸标签名补 `//` 前缀 |
| `<result name="RET_CODE">` 空值 | `<result name="//RET_CODE"></result>` | 保留空预期值（匹配任意值的情况） |
| `<Verify>` 直接包含 `<result>` | `<Verify>` 包含 `<result>` | 基本一致，但需调整 XPath 格式 |
| `<Verify>` 包含 `<value>`（SQL 预期） | `<Verify>` 包含 `<value>` | 直接保留 |
| `<Verify>` 值是 `*` | 转换为 `runtime_verify` 或跳过 | 对于 `*`（跳过验证），可以移除 Verify 节点，迁移报告记录 |
| `<result>` 中 `isHeader="True"` 属性 | 新 DSL 的 `result` 元素保留 `isHeader` 属性 | 直接保留 |

### 2.5 `<save>` 元素（关键差异：方向反转）

| 旧 DSL | 新 DSL | 映射方式 |
|--------|--------|---------|
| `<key name="//Sys_Head/SEQ_NO">case_seqno</key>` | `<key name="case_seqno">//Sys_Head/SEQ_NO</key>` | **交换 key 的 name 和 content**：旧 DSL 中 `<key name="LOCATOR">VAR_NAME</key>` → 新 DSL 中 `<key name="VAR_NAME">LOCATOR</key>` |
| `<key name="/Reply_Msg/Body/LEDGER_BAL">LEDGER_BAL1</key>` | `<key name="LEDGER_BAL1">/Reply_Msg/Body/LEDGER_BAL</key>` | 同上，XPath 从 name 移到内容 |
| `<key name="">SUB_GROUP_NO</key>`（SQL 提取，name 为空） | `<key name="SUB_GROUP_NO"></key>` | 保留变量名，locator 为空（SQL 场景默认提取首行首列） |
| `<save>` 中有 `locator` 属性属性不存在 | 新 DSL 支持 `<key name="var" locator="$.path">` | 如果旧 DSL 的 key content 以 `$.` 开头，优先使用 `locator` 属性 |

### 2.6 HTTP 请求元素

| 旧 DSL | 新 DSL | 映射方式 |
|--------|--------|---------|
| `<Action type="damper_set">` 内联 JSON `{body, headers, querystring}` | `<body>`、`<header>`、`<queryString>` 作为 `<step>` 子元素 | **拆分内联 JSON 为独立元素**：解析 JSON 结构，提取 `body`、`headers`、`querystring` 分别映射 |
| `<Action ip="..." port="..." api="..." method="...">` 属性 | 新 DSL 的相同属性加上 `server="..."` 命名服务 | 直接保留 `ip`、`port`、`api`、`method` 属性 |
| 无 `<body>` 显式子元素 | `<body>` 作为 `<step>` 子元素 | 从内联 JSON 的 `body` 字段生成 |
| 无 `<header>` 显式子元素 | `<header name="HeaderName">value</header>` | 从内联 JSON 的 `headers` 对象生成 |
| 无 `<queryString>` 显式子元素 | `<queryString name="key">value</queryString>` | 从内联 JSON 的 `querystring` 对象生成 |

### 2.7 `xml_setting` 映射

`<preActions>` 中的 `<Action type="xml_setting" tag="TAG_NAME">` 用于预定义模板中特定标签的默认值：

| 旧 DSL | 新 DSL 等价 | 映射方式 |
|--------|------------|---------|
| `<Action type="xml_setting" tag="TRAN_DATE"><value type="date">now.YYYYMMDD</value></Action>` | 无直接等价 | 转换为 `<setup>` 中的运行时变量赋值 `<key name="TRAN_DATE">{{CurrentDate}}</key>` 或者作为内联上下文变量 |
| `<Action type="xml_setting" tag="SERVICE_CODE"><value type="string">SVR_INQUIRY</value></Action>` | 无直接等价 | 转换为模板变量默认值注入 |
| `type="date"` 且值 `now.YYYYMMDD` | 变量 `{{CurrentDate}}` 由系统自动生成 | 迁移工具输出警告标注，提示用户确认当前日期变量已正确生成 |
| `type="string"` | 字符串常量 | 直接保持 |

设计决策：`xml_setting` 在旧 DSL 中由 JMeter 的 UserParameters 在步骤循环外生成，然后在模板参数化时替换。新 DSL 中不存在全局预设置的概念，这些值需要在 `<setup>` 中通过 `runtime_verify` 的变量赋值来模拟，或者直接在步骤的 `<value>` 中使用管道变量。

### 2.8 标签 / flag 属性

| 旧 DSL | 新 DSL | 映射方式 |
|--------|--------|---------|
| `flag="core full"` | `flags="core full"` | 属性重命名 `flag` → `flags`，值不变 |
| `flag="core full 1400"` | `flags="core full 1400"` | 同上 |

### 2.9 `<case>` 多 case 文件拆分

对于同一文件包含多个 `<case>` 的情况：

```xml
<root>
  <preActions>...</preActions>
  <case id="0001" tittle="case1" ...>...</case>
  <case id="0002" tittle="case2" ...>...</case>
</root>
```

→ 拆分为两个独立文件：

```
testcase/case1/case1.xml
testcase/case2/case2.xml
```

- 每个文件 `<preActions>` 中提取的 `xml_setting` → 注入到每个拆分后的 `<setup>` 中
- case 的 `id` 属性用于消除同名 tittle 冲突

---

## 3. 方案选择

**全面论证参见 [1.1 方案选择](#11-方案选择离线迁移工具-vs-运行时-legacyloader)**。

### 3.1 迁移工具架构

```
┌─────────────────────┐
│   scan source dir   │
│  (projects/*/testcase)│
└─────────┬───────────┘
          ▼
┌─────────────────────┐
│  parse XML (<root>)  │
│  extract <case> list │
└─────────┬───────────┘
          ▼
┌──────────────────────────────────────┐
│  for each <case>:                     │
│  1. resolve preActions                │
│  2. map each <step> to new format     │
│     - move <value> to step level      │
│     - swap save key direction         │
│     - wrap <result> in <Verify>       │
│     - handle Action type              │
│  3. split multiple cases              │
│  4. collect warnings/errors           │
└─────────┬────────────────────────────┘
          ▼
┌─────────────────────┐
│  write output files  │
│  + generate report   │
└─────────────────────┘
```

### 3.2 核心处理原则

1. **保语义，不保格式**：转换后语法不同，运行时行为等价
2. **不静默丢弃**：任何不支持的旧结构都必须输出警告
3. **文件级边界清晰**：每个 case 输出独立文件，保持原子性
4. **可重入**：多次运行迁移工具应产出相同结果

---

## 4. 验收标准

| # | 验收项 | 验证方式 |
|---|--------|---------|
| 1 | 转换后用例不需要 Handler 理解旧 XML 结构 | 任一转换后的 XML 可被 `ParseStep` 成功解析 |
| 2 | 一个旧文件中的多个 case 全部保留且可独立报告 | 旧文件含 N 个 case → 输出 N 个独立 XML 文件 |
| 3 | 不支持的旧动作不会被静默忽略 | 迁移报告中包含 WARNING 条目对应每个 `test`/`if` 动作 |
| 4 | 代表性迁移样本通过新引擎 dry-run | `engine --test-base ./output --dry-run` 零报错 |
| 5 | 迁移报告包含成功、警告、失败及原文件位置 | 报告 JSON 包含 `success`, `warnings`, `errors` 数组，每个条目有 `source_file` 和 `line` |
| 6 | 旧版 save 方向反转正确 | 旧 `<key name="XPath">var</key>` → 新 `<key name="var">XPath</key>` |

### 4.1 关键样本清单

以下文件覆盖所有差异化场景，应作为迁移工具的验证样本：

| 样本文件 | 覆盖场景 |
|---------|---------|
| `Core/testcase/010101/test_010101.xml` | `test`、`if` 等不支持动作 |
| `BIL/port_7777/031005/test_031005.xml` | `preActions` + `xml_setting` |
| `Sample/testcase/010101/test_010101.xml` | 旧 save 方向反转、多个 case 文件、damper_set HTTP 内联 JSON |
| `Core/testcase/054202/test_054202.xml` | 同一文件的多个 case 拆分 |
| `Core/testcase/020524/test_020524.xml` | SQL 步骤 + 变量引用 `{{SUB_GROUP_NO}}` |
| `Core/testcase/052654/test_052654.xml` | 带 `//Body/` 完整 XPath 的 value |
| `Core/testcase/020127/test_020127.xml` | 保存 XPath `/Reply_Msg/Body/LOST_NO` + 变量链 |

---

## 5. 额外约束

### 5.1 输出文件命名规则

格式：`{trancode}_{id编号}_{title片段（前 30 字符）}.xml`

其中：
- `trancode`：取自 `<case>` 的 `trancode` 属性（如果 case 没有则用默认值 `UNKNOWN`）
- `id编号`：取自 `<case>` 的 `id` 属性（`0001`、`0002` 等）
- `title片段`：取自 `<case>` 的 `tittle` 或 `title`，清理特殊字符

输出目录名与文件名一致（满足新 DSL 每个 case 独立目录的要求）。

冲突处理：如果生成的目录名冲突，优先使用 `id` 编号区分。

### 5.2 稳定唯一标识生成

- 每个迁移后的 case 文件自动分配 UUID v4 作为 `guid` 属性
- 通过 `id` + `trancode` + `tittle` 计算确定性哈希作为回退方案
- `guid` 写入文件属性，确保多次运行 results 稳定

### 5.3 输出格式验证

所有转换后的文件满足以下条件：

1. 可被新引擎 `ParseStep` 解析（测试通过 `ParseStep` 每个 step）
2. 可通过 `engine --test-base <output> --dry-run` 验收
3. XML 通过格式良好性检查（well-formed）
4. `<case>` 元素必须有 `guid` 和 `title` 属性

### 5.4 多 case 文件处理顺序

```
对于同一文件中的多个 <case>：
1. 共享的 <preActions> → 复制到每个 case 的 <setup>
2. 每个 case 独立生成 guid
3. 文件名规则：{trancode}_{id}_{title_简写}.xml
4. 目录名与文件名一致
```

---

## 6. 不支持的旧动作

| 旧动作 | 出现频率 | 诊断级别 | 替代方案 | 迁移行为 |
|--------|---------|---------|---------|---------|
| `<Action type="test">` | 低（仅 test 用例） | WARNING | 新 DSL 无等价 `test` 动作。如果功能是"打印日志/标记步骤"，可用 `runtime_verify` 代替 | 删除此 step，报告标记位置和原内容 |
| `<Action type="if" startOrEnd="start/end">` | 低（仅 test 用例） | WARNING | 新 DSL 无等价条件分支。如果测试需要条件，应在测试数据层处理 | 删除此 step 及其中间分支步骤，报告标记被删除的范围 |
| `<preActions>` / `xml_setting` | 高（几乎所有 Core 用例） | INFO | 不直接等价。旧 DSL 设置模板默认值，新 DSL 在 `<setup>` 中注入变量 | 转换为 `<setup>` 中的变量声明，并输出 INFO 提示用户检查 |
| `<key name="XPath">varName</key>`（旧 save 方向） | 高（几乎所有用例） | 自动转换 | N/A（自动交换方向） | 自动完成，迁移报告 INFO 级别记录 |
| `<value>` 在 `<Action>` 内（旧位位置） | 高（几乎所有用例） | 自动转换 | N/A（自动上移） | 自动完成，迁移报告 INFO 级别记录 |
| `<result>` 在 `<step>` 下（不在 `<Verify>` 内） | 中 | WARNING | 需要包裹到 `<Verify>` 内部 | 自动包裹，WARNING 提示格式调整 |
| 裸 XPath（无 `//` 前缀） | 高 | 自动转换 | 自动补全 `//` | 自动完成 |
| DAMPER HTTP 内联 JSON（非标准格式） | 中 | 自动转换 | 拆分为 `<body>`、`<header>`、`<queryString>` | 自动拆分，INFO 记录 |
| `flag` 单数属性 | 高 | 自动转换 | 重命名为 `flags` | 自动完成 |
| `_` 下划线变量（JMeter 内部变量） | 高 | 自动处理 | N/A | 保持，引擎自动处理 `CurrentDate_1` 等变量 |
| 带引号的 result 值 `"ok"` | 中 | INFO | 新 DSL 期望裸值 `ok` | 去除多余引号，INFO 记录 |

---

## 附录 A：转换示例

### A.1 简单 XML Set 用例

**旧 DSL：**
```xml
<root>
<case id="0001" tittle="010101_001" trancode="010101" flag="core full">
  <step desc="步骤1">
    <Action type="xml_set" trancode="010101">
      <value name="BASE_ACCT_NO">621753030000048531</value>
    </Action>
    <Verify>
      <result name="RET_CODE">000000</result>
    </Verify>
    <save>
      <key name="//Sys_Head/SEQ_NO">case_seqno</key>
    </save>
  </step>
</case>
</root>
```

**新 DSL：**
```xml
<case guid="a1b2c3d4-..." title="010101_001" flags="core full">
  <action>
    <step desc="步骤1">
      <Action type="xml_set" trancode="010101" />
      <value name="//BASE_ACCT_NO">621753030000048531</value>
      <Verify>
        <result name="//RET_CODE">000000</result>
      </Verify>
      <save>
        <key name="case_seqno">//Sys_Head/SEQ_NO</key>
      </save>
    </step>
  </action>
</case>
```

### A.2 带 preActions 的用例

**旧 DSL：**
```xml
<root>
<preActions>
  <Action type="xml_setting" tag="TRAN_DATE">
    <value type="date">now.YYYYMMDD</value>
  </Action>
  <Action type="xml_setting" tag="SERVICE_CODE">
    <value type="string">SVR_INQUIRY</value>
  </Action>
</preActions>
<case id="0001" tittle="050102" trancode="050102" flag="core full 1400">
  <step desc="步骤2">
    <Action type="xml_set" trancode="050102">
      <value name="CLIENT_NO">1016393492</value>
    </Action>
    <Verify>
      <result name="RET_CODE">000000</result>
    </Verify>
  </step>
</case>
</root>
```

**新 DSL：**
```xml
<case guid="..." title="050102" flags="core full 1400">
  <setup>
    <!--  INFO: 从旧 preActions 转换 - xml_setting tag=TRAN_DATE value=now.YYYYMMDD  -->
    <step desc="设置系统日期变量">
      <Action type="runtime_verify" />
      <!--  TRAN_DATE 将由引擎自动使用 {{CurrentDate}} 代替  -->
    </step>
    <!--  INFO: 从旧 preActions 转换 - xml_setting tag=SERVICE_CODE value=SVR_INQUIRY  -->
  </setup>
  <action>
    <step desc="步骤2">
      <Action type="xml_set" trancode="050102" />
      <value name="//CLIENT_NO">1016393492</value>
      <Verify>
        <result name="//RET_CODE">000000</result>
      </Verify>
    </step>
  </action>
</case>
```

### A.3 旧 save 方向反转

**旧 DSL：**
```xml
<save>
  <key name="//Sys_Head/SEQ_NO">case_seqno</key>
  <key name="//Sys_Head/BRANCH_ID">case_branch_id</key>
  <key name="/Reply_Msg/Body/ACCT_ARRAY/Row/LEDGER_BAL">LEDGER_BAL1</key>
</save>
```

**新 DSL：**
```xml
<save>
  <key name="case_seqno">//Sys_Head/SEQ_NO</key>
  <key name="case_branch_id">//Sys_Head/BRANCH_ID</key>
  <key name="LEDGER_BAL1">/Reply_Msg/Body/ACCT_ARRAY/Row/LEDGER_BAL</key>
</save>
```

### A.4 多 case 文件拆分

**旧 DSL（单个文件）：**
```xml
<root>
<preActions>...</preActions>
<case id="0001" tittle="有效利率查询拆入" trancode="054202" flag="core full 1400">
  ...步骤...
</case>
<case id="0002" tittle="有效利率查询拆出" trancode="054202" flag="core full 1400">
  ...步骤...
</case>
</root>
```

**新 DSL（两个独立文件）：**
```
testcase/054202_0001_有效利率查询拆入/054202_0001_有效利率查询拆入.xml
testcase/054202_0002_有效利率查询拆出/054202_0002_有效利率查询拆出.xml
```

### A.5 HTTP damper_set 内联 JSON 拆分

**旧 DSL：**
```xml
<Action type="damper_set" ip="192.168.99.100" port="1080" api="/some/path3" method="POST">
  {"headers":{"Content-Type":"application/json","otherHeader":"otherValue"},
   "body":{"age":"{{case_seqno}}","name":"geek","result":"ok"},
   "querystring":{}}
</Action>
<Verify>
  <result name="result">"ok"</result>
</Verify>
```

**新 DSL：**
```xml
<Action type="http" server="MOCK" method="POST" api="/some/path3" />
<header name="Content-Type">application/json</header>
<header name="otherHeader">otherValue</header>
<body>{"age":"{{case_seqno}}","name":"geek","result":"ok"}</body>
<queryString></queryString>
<Verify>
  <result name="$.result">ok</result>
</Verify>
```

### A.6 不支持的 `if`/`test` 动作

**旧 DSL：**
```xml
<step desc="步骤1">
  <Action type="test" key="hello" value=""></Action>
</step>
<step desc="if分支开始">
  <Action type="if" startOrEnd="start">
    <expression>1==1</expression>
  </Action>
</step>
```

**新 DSL 输出：**
迁移报告:
```json
{
  "warnings": [
    {
      "source_file": "Core/testcase/010101/test_010101.xml",
      "line": 5,
      "case_tittle": "010101_001",
      "message": "Unsupported action type 'test'. Step removed.",
      "detail": "Action type=test key=hello value="
    },
    {
      "source_file": "Core/testcase/010101/test_010101.xml",
      "line": 9,
      "case_tittle": "010101_001",
      "message": "Unsupported action type 'if'. Step and enclosed branch steps removed.",
      "detail": "Action type=if startOrEnd=start expression=1==1"
    },
    ...
  ]
}
```
