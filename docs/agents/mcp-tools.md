# MCP 工具使用指南

> 本项目有两个知识图谱 MCP 工具。**核心规则**（每次必读）见根目录 `AGENTS.md` 的"MCP 工具核心规则"段；本文档提供**详细使用说明**（按需读取）。

---

## GitNexus — Code Intelligence

本项目被 GitNexus 索引为 **y-daq**（3494 symbols, 6905 relationships, 154 execution flows）。

> 若 GitNexus 工具提示索引过期，先在终端运行 `npx gitnexus analyze`。

### Always Do

- **修改符号前必须运行影响分析**：`gitnexus_impact({target: "symbolName", direction: "upstream"})`，并向用户报告影响范围（直接调用者、受影响流程、风险等级）。
- **提交前必须运行** `gitnexus_detect_changes()` 验证改动只影响预期符号和执行流。
- **HIGH 或 CRITICAL 风险必须警告用户**，再决定是否继续编辑。
- 探索陌生代码时，用 `gitnexus_query({query: "concept"})` 按执行流查找，而非 grep。
- 需要符号完整上下文（调用者/被调用者/参与的执行流）时，用 `gitnexus_context({name: "symbolName"})`。

### Never Do

- **绝不**在未运行 `gitnexus_impact` 的情况下编辑函数/类/方法。
- **绝不**忽略 HIGH 或 CRITICAL 风险警告。
- **绝不**用 find-and-replace 重命名符号 —— 用 `gitnexus_rename`（理解调用图）。
- **绝不**在未运行 `gitnexus_detect_changes()` 的情况下提交改动。

### Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/y-daq/context` | Codebase overview, check index freshness |
| `gitnexus://repo/y-daq/clusters` | All functional areas |
| `gitnexus://repo/y-daq/processes` | All execution flows |
| `gitnexus://repo/y-daq/process/{name}` | Step-by-step execution trace |

### CLI Skill Files

| Task | Skill file |
|------|-----------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

### Area Skills（按功能域）

| Area | Symbols | Skill file |
|------|---------|-----------|
| Three_hole | 156 | `.claude/skills/generated/three-hole/SKILL.md` |
| Go | 90 | `.claude/skills/generated/go/SKILL.md` |
| Stores | 82 | `.claude/skills/generated/stores/SKILL.md` |
| Manager | 74 | `.claude/skills/generated/manager/SKILL.md` |
| Views | 60 | `.claude/skills/generated/views/SKILL.md` |
| Driver | 48 | `.claude/skills/generated/driver/SKILL.md` |
| Calibration | 29 | `.claude/skills/generated/calibration/SKILL.md` |
| Storage | 18 | `.claude/skills/generated/storage/SKILL.md` |
| App | 16 | `.claude/skills/generated/app/SKILL.md` |
| Components | 14 | `.claude/skills/generated/components/SKILL.md` |
| Types | 7 | `.claude/skills/generated/types/SKILL.md` |
| Main | 7 | `.claude/skills/generated/main/SKILL.md` |
| Scanner | 4 | `.claude/skills/generated/scanner/SKILL.md` |
| Runtime | 4 | `.claude/skills/generated/runtime/SKILL.md` |
| Composables | 4 | `.claude/skills/generated/composables/SKILL.md` |

---

## code-review-graph

**重要：本项目有知识图谱。探索代码时优先用 graph 工具，而非 Grep/Glob/Read。** Graph 更快、更省 token，且能提供文件扫描无法给出的结构上下文（调用者、依赖者、测试覆盖）。

### 何时优先用 graph 工具

- **探索代码**：`semantic_search_nodes` 或 `query_graph` 替代 Grep
- **理解影响**：`get_impact_radius` 替代手动追踪 import
- **代码审查**：`detect_changes` + `get_review_context` 替代读整个文件
- **查找关系**：`query_graph` 的 callers_of / callees_of / imports_of / tests_for
- **架构问题**：`get_architecture_overview` + `list_communities`

仅在 graph 不覆盖所需信息时回退到 Grep/Glob/Read。

### 工具速查

| Tool | Use when |
|------|----------|
| `detect_changes` | 审查代码改动 —— 给出风险评分分析 |
| `get_review_context` | 需要源码片段做审查 —— 省 token |
| `get_impact_radius` | 理解改动的爆炸半径 |
| `get_affected_flows` | 查找受影响的执行路径 |
| `query_graph` | 追踪调用者/被调用者/imports/tests/依赖 |
| `semantic_search_nodes` | 按名称或关键词查找函数/类 |
| `get_architecture_overview` | 理解高层代码库结构 |
| `refactor_tool` | 规划重命名、查找死代码 |

### 工作流

1. 文件变更时 graph 通过 hooks 自动更新
2. 用 `detect_changes` 做代码审查
3. 用 `get_affected_flows` 理解影响
4. 用 `query_graph` pattern="tests_for" 检查测试覆盖
