# ECC Skills & Agents

> ECC agents 和 skills 安装在系统级（`~/.claude/` 和 `~/.opencode/`）。通过 Task 工具调用。

## 可用 Agents

| Agent | 用途 |
|-------|------|
| `/plan` | 新功能实现规划 |
| `/architect` | 架构设计决策 |
| `/code-review` | 代码质量审查 |
| `/go-review` | Go 代码专项审查 |
| `/go-build` | Go 构建错误修复 |
| `/go-test` | Go 测试编写/执行 |
| `/rust-review` | Rust 代码审查 |
| `/rust-build` | Rust 构建修复 |
| `/rust-test` | Rust 测试执行 |
| `/python-review` | Python 代码审查 |
| `/typescript-review` | TypeScript 专项审查 |
| `/database-review` | 数据库/存储设计审查 |
| `/tdd` | 测试驱动开发流程 |
| `/verify` | 验证循环（lint → test → build） |
| `/refactor-clean` | 死代码清理 |
| `/security` | 安全审查 |
| `/checkpoint` | 保存检查点 |
| `/save-session` | 保存 session 摘要 |

## 验证规范

- 修改 Go 后：`cd src && golangci-lint run ./internal/...` + `go build ./...`
- 修改前端后：`cd src/frontend && npm run lint` + `npm run build`
- 代码提交前：运行 `/verify`
- 复杂功能开发前：运行 `/plan`

## Issue Tracker

Issues tracked in GitHub Issues at `wuzy0325/Y-DAQ`。External PRs 作为 triage surface。详见 `docs/agents/issue-tracker.md`。

## Triage Labels

使用默认 canonical labels：`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`。详见 `docs/agents/triage-labels.md`。

## Domain Docs

Single-context layout：`CONTEXT.md` 和 `docs/adr/` 在 repo root。详见 `docs/agents/domain.md`。
