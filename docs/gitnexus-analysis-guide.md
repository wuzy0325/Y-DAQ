# GitNexus 代码分析和 Bug 查找指南

## 概述

GitNexus 是一个零服务器的代码智能引擎，可以帮助你深入分析代码模块的业务逻辑、查找潜在问题，并与 Claude Code 深度集成。

## 安装和配置

### 1. 安装 GitNexus
```bash
# 全局安装
npm install -g gitnexus

# 或使用 npx（推荐）
npx gitnexus --version
```

### 2. 启动本地服务器
```bash
# 启动本地服务器
npx gitnexus serve

# 后台运行
nohup npx gitnexus serve > /tmp/gitnexus.log 2>&1 &
```

### 3. 配置 Claude Code
已在 `~/.claude/settings.json` 中配置了 MCP 服务器：

```json
{
  "mcpServers": {
    "gitnexus": {
      "command": "npx",
      "args": ["gitnexus", "serve"]
    }
  }
}
```

## 基本用法

### 1. 简单分析
```bash
# 分析当前目录
npx gitnexus analyze .

# 指定分析深度
npx gitnexus analyze . --depth=2

# 分析特定模块
npx gitnexus analyze ./internal/app
```

### 2. Web UI 可视化分析
```bash
# 启动 Web 服务
npx gitnexus serve

# 访问 http://localhost:3000 或 https://gitnexus.vercel.app
```

## 业务逻辑分析方法

### 1. 数据流分析
```bash
# 分析数据流向和依赖关系
npx gitnexus analyze . --focus="data-flow"

# 示例分析流程：
# 1. 识别数据输入源（设备、API等）
# 2. 跟踪数据处理管道
# 3. 分析数据存储和输出
```

### 2. 架构分析
```bash
# 分析模块架构和依赖关系
npx gitnexus analyze . --focus="architecture"

# 分析内容：
# - 模块间耦合度
# - 依赖方向是否合理
# - 是否存在循环依赖
# - 接口设计是否清晰
```

### 3. 业务流程分析
```bash
# 分析特定业务模块的业务逻辑
npx gitnexus analyze ./internal/app --module="业务逻辑分析"

# 分析要点：
# 1. 业务起点和终点
# 2. 核心处理逻辑
# 3. 异常处理机制
# 4. 状态管理方式
```

### 4. 模式识别
```bash
# 识别设计模式和反模式
npx gitnexus analyze . --focus="patterns"

# 常见模式：
# - 单例模式
# - 观察者模式
# - 工厂模式
# - 适配器模式
# - 反模式：上帝类、魔法字符串等
```

## Bug 查找技巧

### 1. 空指针检查
```bash
# 查找可能的空指针访问
npx gitnexus analyze . --focus="null-pointer"

# 常见空指针风险：
# - 未检查的返回值
# - nil 调用方法
# - 未初始化的变量使用
```

### 2. 资源泄漏
```bash
# 查找未关闭的资源
npx gitnexus analyze . --focus="resource-leak"

# 检查类型：
# - 文件句柄
# - 数据库连接
# - 网络连接
# - goroutine
# - 互斥锁
```

### 3. 并发问题
```bash
# 查找并发相关的问题
npx gitnexus analyze . --focus="concurrency"

# 潜在问题：
# - 数据竞争
# - 死锁风险
# - 重复加锁
# - 条件竞争
```

### 4. 错误处理
```bash
# 分析错误处理是否完善
npx gitnexus analyze . --focus="error-handling"

# 检查点：
# - 是否所有错误都被处理
# - 错误信息是否清晰
# - 是否有错误恢复机制
# - 日志记录是否完整
```

## 实际案例分析

### 案例示例：YX-DAQ 项目分析

#### 发现的问题 1：空指针风险
```go
// 问题代码（app.go:154-157）
if mcID == "" {
    slog.Warn("三孔测试: 未指定运动控制器，将使用第一个已连接的控制器")
}
// 问题：如果 mcID 为空且没有已连接的控制器，后续 MoveTo 调用会失败
```

#### 发现的问题 2：资源清理不完整
```go
// 问题代码（app.go:196-202）
a.publishCancel <- struct{}{}
// ...
for a.threeHoleService.GetStatus().Status == types.TraversalStatusRunning && time.Now().Before(shutdownDeadline) {
    time.Sleep(50 * time.Millisecond)
}
// 问题：只等待了 3 秒，如果任务未完成可能强制终止
```

#### 发现的问题 3：竞态条件
```go
// 问题代码（app.go:84-86）
a.calibService.SetDataGetter(func(deviceID string, channelIndex int) (float64, bool) {
    return a.deviceManager.GetChannelValue(deviceID, channelIndex)
})
// 问题：在服务初始化过程中，如果设备管理器尚未初始化完成，可能返回无效数据
```

## 高级用法

### 1. 自定义分析规则
创建 `.gitnexus-rules.json`：
```json
{
  "rules": [
    {
      "name": "custom-check",
      "pattern": "TODO",
      "severity": "warning"
    },
    {
      "name": "log-check",
      "pattern": "log\\.Error",
      "severity": "info"
    }
  ]
}
```

运行自定义分析：
```bash
npx gitnexus analyze . --rules=.gitnexus-rules.json
```

### 2. 输出格式控制
```bash
# 输出为 Markdown
npx gitnexus analyze . --output="analysis.md"

# 输出 JSON（如果支持）
npx gitnexus analyze . --format="json"
```

### 3. 与 CI/CD 集成
```bash
# 在 CI 流程中运行分析
npx gitnexus analyze . --quiet > analysis-summary.txt

# 检查是否有严重问题
if grep -q "ERROR:" analysis-summary.txt; then
    exit 1
fi
```

## 与 Claude Code 集成

### 1. 自动分析
GitNexus 已集成到 Claude Code 的 MCP 配置中，可以：
- 自动获得代码分析和建议
- 实时反馈代码质量问题
- 提供上下文相关的代码洞察

### 2. 智能提示
当你编写代码时，GitNexus 可以：
- 识别潜在的代码问题
- 提供重构建议
- 发现设计优化机会

## 最佳实践

### 1. 分析时机
- **代码提交前**：确保代码质量
- **代码审查时**：提供专业的代码分析
- **重构前**：了解现有架构和问题
- **新功能开发**：识别集成风险

### 2. 分析策略
1. **渐进式分析**：从核心模块开始，逐步扩展
2. **定期分析**：定期运行全量分析，监控代码质量变化
3. **问题跟踪**：建立问题列表，跟踪解决进度

### 3. 团队协作
- 共享分析规则
- 建立代码质量标准
- 定期分享分析结果
- 集成到开发流程

## 常见问题解答

### Q: GitNexus 与静态分析工具的区别？
A: GitNexus 专注于代码结构和业务逻辑分析，能理解代码的业务含义，而传统的静态分析工具主要关注语法和类型检查。

### Q: 分析性能如何？
A: GitNexus 是客户端工具，分析性能取决于项目大小和复杂度。对于大型项目，建议使用 `--depth` 参数控制分析深度。

### Q: 如何处理误报？
A: 可以通过自定义规则调整分析严格度，忽略特定模式或添加业务特定的验证规则。

## 参考资料

- [GitNexus GitHub 仓库](https://github.com/abhigyanpatwari/GitNexus)
- [在线 Web UI](https://gitnexus.vercel.app)
- [MCP 协议文档](https://modelcontextprotocol.io/)

---

*最后更新：2026-05-02*