# CLAUDE.md

Behavioral guidelines for working on YX-DAQ (Wails v2 + Go 1.23 + Vue 3 + TypeScript).

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No error handling for impossible scenarios.
- Don't add comments just to explain WHAT code does — well-named identifiers already do that.

## 3. Surgical Changes

**Touch only what you must. Clean up only what you created.**

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If your changes create orphaned imports/variables, remove them.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
```

Always verify with `go build ./...` and `cd frontend && npm run build` before claiming completion.

## 5. Project Rules

**必须遵守以下项目文档**：
- `docs/engineering/design-principles.md` —— 设计原则与架构规范（上位规范，所有编码决策的根约束）
- `docs/engineering/coding-standards.md` —— 编码规范（Go + 前端）
- `docs/engineering/project-layout-rules.md` —— 目录与文件组织
- `docs/engineering/dev-guide.md` —— 开发约定（补充参考）

核心约束速查：
- `internal/` 按业务领域分包，禁止循环依赖
- Go 错误包装用 `%w`，日志用 `log.Printf`
- 前端 Store 中 Wails 绑定必须静态 import
- 禁止手动编辑 `frontend/wailsjs/` 目录
- 前端样式必须 `<style lang="scss" scoped>`
- 所有 UI 字符串用中文
- 修改后验证：`golangci-lint run ./internal/...` + `go build ./...`（Go）
- 修改前端后验证：`cd frontend && npm run lint` + `npm run build`
- 复杂功能前可用 `/plan`、`/architect` agents 做设计
