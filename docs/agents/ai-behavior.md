# AI 行为准则

> 适用于所有 AI Agent（Claude Code / Trae / 其他）。核心约束速查见根目录 `AGENTS.md`，本文档规定**如何思考与执行**。

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

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

Always verify with `cd src && go build ./...` and `cd src/frontend && npm run build` before claiming completion.

## 5. 验证规范

- 修改 Go 后：`cd src && golangci-lint run ./internal/...` + `go build ./...`
- 修改前端后：`cd src/frontend && npm run lint` + `npm run build`
- 代码提交前：运行 `/verify`
- 复杂功能开发前：运行 `/plan`
