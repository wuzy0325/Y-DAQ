# YX-DAQ UI 设计规范

> 本文档定义前端视觉与交互规范，覆盖色彩、字体、布局、组件使用及 Element Plus 定制。所有前端代码必须遵守。架构与目录结构见 `architecture.md`，编码规范见 `coding-standards.md`。

---

## 一、设计原则

### 1.1 深色科技风（Dark Tech）

YX-DAQ 采用**深色科技风**作为唯一设计方向：

- **深空背景**：以 `#0a0a1a` 为基底，营造沉浸式数据环境
- **霓虹强调色**：紫色（`#b829ff`）与青色（`#00f5ff`）作为品牌双主色，用于交互高亮与关键数据
- **玻璃拟态（Glassmorphism）**：卡片与面板使用半透明背景 +  backdrop-filter 模糊，增强层级感
- **发光效果**：重要状态、活跃元素使用柔和 box-shadow 霓虹光晕，替代传统的实色边框
- **数据优先**：UI 不过度装饰，所有视觉效果服务于数据可读性与操作效率

### 1.2 一致性优先

- **单一色彩系统**：禁止混用多套色值（见 §2 色彩系统）
- **单一卡片系统**：所有卡片/面板优先使用 `GlassCard` 组件，禁止手动复制相同样式
- **单一间距系统**：所有间距必须从 spacing token 选取，禁止随意写 `13px`、`17px` 等魔数
- **单一状态语言**：同义词必须统一（如“连接/断开”而非“在线/离线”）

### 1.3 反馈即时

- 所有耗时操作（>300ms）必须显示加载状态（`el-button :loading`、旋转图标、进度条）
- 所有操作结果必须通过 `ElMessage` 或状态指示器反馈给用户
- 状态变化必须有视觉过渡（颜色渐变、尺寸缩放、发光脉冲），禁止瞬间跳变

---

## 二、色彩系统（唯一权威）

> **警告**：当前代码中存在三套 competing 色彩系统（`variables.scss`、`theme-variables.scss`、`global.scss`）。
> 本规范以 `variables.scss` 为**唯一权威来源**，其余两套仅作向后兼容，新代码必须严格使用以下 token。

### 2.1 核心色板

| Token | SCSS 变量 | 色值 | 用途 |
|-------|-----------|------|------|
| 主品牌色 | `$color-primary` | `#b829ff` | Logo、导航激活、品牌标识 |
| 主品牌发光 | `$color-primary-glow` | `rgba(184,41,255,0.6)` | 阴影、光晕 |
| 强调色 | `$color-accent` | `#00f5ff` | 按钮悬停、链接、数据高亮、Element Primary |
| 强调发光 | `$color-accent-glow` | `rgba(0,245,255,0.6)` | 发光边框、脉冲动画 |
| 成功色 | `$color-success` | `#00ff88` | 运行中、已连接、成功提示 |
| 警告色 | `$color-warning` | `#ffaa00` | 警告、注意、暂停 |
| 危险色 | `$color-danger` | `#ff3366` | 错误、急停、断开连接 |
| 信息色 | `$color-info` | `#00aaff` | 提示信息、次要强调 |

**轴色规范（必须遵守）**：

| 轴 | 颜色 | Token |
|----|------|-------|
| X | 紫色 | `$color-primary` / `#b829ff` |
| Y | 青色 | `$color-accent` / `#00f5ff` |
| Z | 绿色 | `$color-success` / `#00ff88` |
| U | 橙色 | `$color-warning` / `#ffaa00` |

### 2.2 背景色板

| Token | SCSS 变量 | 色值 | 用途 |
|-------|-----------|------|------|
| 主背景 | `$bg-primary` | `#0a0a1a` | 页面底色 |
| 玻璃背景 | `$glass-bg` | `rgba(255,255,255,0.06)` | 卡片、面板默认背景 |
| 玻璃悬停 | `$glass-bg-hover` | `rgba(255,255,255,0.10)` | 卡片悬停状态 |
| 玻璃激活 | `$glass-bg-active` | `rgba(255,255,255,0.14)` | 卡片激活/按下状态 |
| Elevated 背景 | `$glass-bg-elevated` | `rgba(255,255,255,0.08)` | 提升层级卡片（如模态内卡片） |
| 三级背景 | `$bg-tertiary` | `rgba(255,255,255,0.03)` | 最弱层级容器 |

### 2.3 文字色板

| Token | SCSS 变量 | 色值 | 用途 |
|-------|-----------|------|------|
| 主文字 | `$text-primary` | `#ffffff` | 标题、重要数据 |
| 次文字 | `$text-secondary` | `rgba(255,255,255,0.8)` | 正文、标签 |
| 三级文字 | `$text-tertiary` | `rgba(255,255,255,0.6)` | 辅助说明、hint |
| 弱化文字 | `$text-muted` | `rgba(255,255,255,0.4)` | 禁用、占位符、单位 |

### 2.4 边框色板

| Token | SCSS 变量 | 色值 | 用途 |
|-------|-----------|------|------|
| 玻璃边框 | `$glass-border` | `rgba(255,255,255,0.12)` | 卡片默认边框 |
| 浅边框 | `$glass-border-light` | `rgba(255,255,255,0.06)` | 内部分隔线、区块边界 |
| 聚焦边框 | `$glass-border-focus` | `rgba(168,85,247,0.6)` | 输入框聚焦、选中态 |

### 2.5 图表色板

| 数据系列 | 颜色 | Token |
|----------|------|-------|
| 系列 1 / Kα | `#b829ff` | `$chart-line-1` |
| 系列 2 / Kβ | `#00f5ff` | `$chart-line-2` |
| 系列 3 / Cps | `#00ff88` | `$chart-line-3` |
| 系列 4 / Cpt | `#ffaa00` | `$chart-line-4` |

### 2.6 使用规则

**必须**：
- 所有颜色引用必须使用 SCSS 变量（如 `$color-primary`），禁止写死十六进制
- Vite 已全局自动注入 `@use "@/assets/styles/variables.scss" as *;`，无需手动 import
- 需要在 JS/TS 中使用的颜色，统一定义到 `frontend/src/constants/colors.ts`（如不存在则新建）

**禁止**：
- 禁止在组件 `<style>` 中写死 `#b829ff`、`#00f5ff` 等色值
- 禁止混用 `variables.scss`、`theme-variables.scss`、`global.scss` 三套的色值
- 禁止在 template 中使用 `style="color: #xxx"` 内联样式（应使用 class + SCSS 变量）

**迁移说明**：
- 现有代码中的硬编码色值应在重构时逐步替换为 SCSS 变量
- `global.scss` 中的 `:root` CSS 变量仅用于 Element Plus 主题覆盖，业务组件不直接引用
- `theme-variables.scss` 为未来多主题预留，当前深色主题下不直接使用

---

## 三、字体排版

### 3.1 字体栈

| 类型 | SCSS 变量 | 值 |
|------|-----------|-----|
| 基础字体 | `$font-family-base` | `"Microsoft YaHei", -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif` |
| 等宽字体 | `$font-family-mono` | `"Microsoft YaHei", 'JetBrains Mono', 'Fira Code', Consolas, 'Courier New', monospace` |

**规则**：
- 所有数值、代码、时间戳必须使用等宽字体，保证对齐
- 中文标签优先使用 `"Microsoft YaHei"`，确保 Windows 环境下清晰显示

### 3.2 字号层级

| Token | 大小 | 字重 | 用途 |
|-------|------|------|------|
| `$font-size-xs` | 11px | 400 | 时间戳、路径、状态标签、单位 |
| `$font-size-sm` | 13px | 400/500 | 正文、表单标签、表格内容 |
| `$font-size-md` / `$font-size-base` | 14px | 500/600 | 卡片标题、导航、按钮文字 |
| `$font-size-lg` | 16px | 600 | 页面小标题、Logo 文字 |
| `$font-size-xl` | 18px | 600 | 区块标题 |
| `$font-size-2xl` | 20px | 600/700 | 大数值显示（`ValueDisplay`） |
| `$font-size-3xl` | 24px | 700 | 页面主标题、仪表盘大数字 |

### 3.3 排版规则

- **行高**：正文 `1.5`，标题 `1.3`，数值 `1.0`
- **数值对齐**：所有动态数值使用 `font-variant-numeric: tabular-nums`，防止宽度跳动
- **文本截断**：路径、长标签必须使用 `text-overflow: ellipsis` + `white-space: nowrap`
- **全中文 UI**：所有界面文本、错误提示、空状态文案必须使用中文

---

## 四、间距系统

### 4.1 基础间距

| Token | 值 | 用途 |
|-------|-----|------|
| `$spacing-xs` | 4px | 图标与文字间距、紧凑内联元素 |
| `$spacing-sm` | 8px | 按钮组间距、表单 item 内部间距 |
| `$spacing-md` | 12px | 卡片 header 与 body 间距、小模块间距 |
| `$spacing-lg` | 16px | 卡片内边距、表单项之间间距 |
| `$spacing-xl` | 24px | 卡片之间间距、区块间距 |
| `$spacing-2xl` | 32px | 大区块间距、页面级 padding |
| `$spacing-3xl` | 48px | 页面边距、布局级间距 |

### 4.2 布局规则

- **页面边距**：`$spacing-lg`（16px）
- **卡片间隙**：`$spacing-lg`（16px）
- **卡片内边距**：`$spacing-lg`（16px）
- **表单行间距**：`$spacing-md`（12px）
- **按钮组间距**：`$spacing-sm`（8px）

**禁止**：
- 禁止写 `margin: 13px`、`padding: 17px` 等非 token 值
- 禁止在 template 中使用 `style="margin-top: 12px"`，应使用 class

---

## 五、圆角与阴影

### 5.1 圆角

| Token | 值 | 用途 |
|-------|-----|------|
| `$border-radius-sm` | 8px | 小按钮、标签、输入框 |
| `$border-radius-md` | 12px | **卡片默认圆角（GlassCard）** |
| `$border-radius-lg` | 16px | 大卡片、模态框 |
| `$border-radius-xl` | 20px | 特殊展示卡片 |
| `$border-radius-2xl` | 24px | 巨型容器 |

**例外**：Element Plus 按钮在 `global.scss` 中被全局覆盖为 `border-radius: 2px`，保持一致。

### 5.2 阴影

| Token | 值 | 用途 |
|-------|-----|------|
| `$shadow-sm` | `0 2px 8px rgba(0,0,0,0.3)` | 轻微提升 |
| `$shadow-md` | `0 4px 16px rgba(0,0,0,0.4)` | 卡片默认 |
| `$shadow-lg` | `0 8px 32px rgba(0,0,0,0.5)` | 悬浮面板、下拉菜单 |
| `$shadow-glass` | `0 8px 32px rgba(0,0,0,0.4), inset 0 1px 0 rgba(255,255,255,0.1)` | **GlassCard 默认阴影** |
| `$shadow-glow-primary` | 多层霓虹紫发光 | 品牌强调元素悬浮 |
| `$shadow-glow-accent` | 多层霓虹青发光 | 交互元素悬浮 |

---

## 六、组件使用规范

### 6.1 GlassCard（核心容器）

`GlassCard` 是所有卡片/面板的**唯一入口**。禁止在 view 中手写 `.glass-panel` 样式。

```vue
<template>
  <GlassCard title="设备状态" icon="📡">
    <template #actions>
      <el-button size="small">刷新</el-button>
    </template>
    <!-- 内容 -->
  </GlassCard>
</template>
```

**Props**：

| Prop | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `title` | `string` | `undefined` | 卡片标题（可选） |
| `icon` | `string` | `undefined` | 标题前的 emoji 图标（可选） |
| `elevated` | `boolean` | `false` | 提升层级（更深的背景与阴影） |

**规则**：
- 每个 view 中的功能区块必须包裹在 `GlassCard` 中
- `elevated` 仅用于模态框内部卡片或需要强层级区分的场景
- `#actions` 插槽放置右上角操作按钮组

### 6.2 StatusIndicator（状态指示）

```vue
<StatusIndicator status="connected" label="设备已连接" :animated="true" />
```

**状态映射**：

| status | 颜色 | 用途 |
|--------|------|------|
| `connected` | `$color-success` | 设备已连接 |
| `disconnected` | `#666`（弱化灰） | 设备未连接 |
| `running` | `$color-accent` | 服务运行中 |
| `warning` | `$color-warning` | 警告状态 |
| `error` | `$color-danger` | 错误状态 |

**规则**：
- `animated` 仅在需要吸引注意时使用（如运行中、警告），禁止全局滥用
- 状态标签文案必须简洁（≤6 个中文字符）

### 6.3 ValueDisplay（数值展示）

```vue
<ValueDisplay :value="pressure" :precision="4" unit="Pa" color="#00f5ff" />
```

**规则**：
- `color` 必须传入 SCSS 变量对应的色值（或轴色）
- `precision` 根据数据精度需求设定，默认 3
- 用于仪表盘、实时数据、测试结果等关键数值

### 6.4 ChartPanel（图表面板）

ECharts 的包装组件。所有图表必须通过此组件渲染。

**规则**：
- 图表 option 中的颜色必须从图表色板（§2.5）选取
- 图表背景必须透明（`backgroundColor: 'transparent'`）
- 坐标轴文字使用 `$text-secondary`，网格线使用 `rgba(255,255,255,0.05)`

### 6.5 表单规范

- 统一使用 `el-form size="small" label-width="80px"`
- 紧凑模式添加 `compact-form` class（减少 `el-form-item` 下边距）
- 表单项横向排列使用 `.form-row`（`display: flex; gap: 24px; align-items: flex-end`）
- 所有输入框下方如有说明文字，使用 `.form-hint` class（`$text-muted`，11px）

### 6.6 表格规范

- 统一使用 `el-table size="small"`
- 表头背景已全局覆盖为 `rgba(255,255,255,0.05)`，无需额外设置
- 行 hover 背景已全局覆盖为 `rgba(0,242,255,0.08)`
- 如需自定义列样式，通过 `:deep(.el-table__cell)` 覆盖，禁止直接修改 Element Plus 源文件

### 6.7 对话框规范

| 场景 | 宽度 | 说明 |
|------|------|------|
| 添加/确认 | 420px | 简单表单或确认操作 |
| 配置编辑 | 600px | 中等复杂度配置 |
| 数据编辑（含表格） | 720px | 需要展示较多字段 |

- 必须设置 `:append-to-body="true"`
- 复杂表单对话框设置 `destroy-on-close`
- 对话框内如需分区，使用 `GlassCard`（非 `el-card`）

---

## 七、Element Plus 定制规范

### 7.1 定制层级

Element Plus 的样式覆盖分为**两层**，禁止在更多地方零散覆盖：

1. **全局覆盖**（`frontend/src/assets/styles/global.scss`）：所有组件通用的深色主题适配
2. **视图级覆盖**（各 `.vue` 文件的 `<style scoped>`）：仅当前视图特有的组件微调

### 7.2 已全局覆盖的组件

以下组件已在 `global.scss` 中完成深色适配，**禁止在视图级重复覆盖相同属性**：

- `el-button`：圆角 2px、大写、深色玻璃背景、悬停反色
- `el-card`：深色背景、青色边框
- `el-input` / `el-input-number`：黑色半透明背景、白色文字
- `el-table`：透明背景、青色 hover、自定义表头
- `el-menu`：深色背景、青色 active 指示器
- `el-dialog`：`#1a1a2e` 背景、青色边框、自定义 header/body/footer
- `el-overlay`：`rgba(0,0,0,0.6)` + `backdrop-filter: blur(4px)`
- `el-popover`：`#1a1a2e` 背景、青色边框
- `el-checkbox`：白色标签、青色 checked 状态

### 7.3 允许视图级覆盖的场景

- `el-tabs` 的 active 颜色需与视图主题色匹配（如 ThreeHoleTestView 使用青色）
- `el-slider` 的轨道颜色需与视图主题色匹配
- `el-progress` 的颜色需与数据系列颜色匹配
- `el-switch` 在特定表格内的颜色微调

**覆盖方式**：

```scss
<style lang="scss" scoped>
.my-view {
  :deep(.el-tabs__active-bar) {
    background-color: $color-accent;
  }
  :deep(.el-tabs__item.is-active) {
    color: $color-accent;
  }
}
</style>
```

---

## 八、状态与反馈

### 8.1 加载状态

| 场景 | 反馈方式 |
|------|----------|
| 按钮触发异步操作 | `el-button :loading="saving"` |
| 页面/区块初始加载 | `el-skeleton` 或自定义骨架屏 |
| 长时间任务（>2s） | 进度条或百分比提示 |
| 表格数据加载 | 表格内部 `v-loading` |

### 8.2 空状态

- 优先使用 `el-empty`（SettingsView 模式）
- 自定义空状态使用 `.empty-state` class：垂直居中、弱化文字颜色、icon + 文案
- 空状态文案必须给出下一步引导（如“暂无设备，点击扫描发现设备”）

### 8.3 错误状态

- **全局错误**：`ElMessage.error('中文错误描述')`，3 秒自动消失
- **表单错误**：`el-form-item` 的 `error` 属性或 rules 校验
- **区块错误**：`el-alert`（closable），用于非阻塞性错误提示
- **致命错误**：对话框阻断，必须用户确认后才能继续

### 8.4 动画规范

| 动画 | 时长 | 用途 |
|------|------|------|
| `$transition-fast` | 150ms | 按钮悬停、颜色变化 |
| `$transition-base` | 250ms | 卡片悬停、展开/收起 |
| `$transition-slow` | 350ms | 模态框出现、页面切换 |
| `statusPulse` | 1.5s infinite | 状态指示器脉冲（运行中/警告） |

---

## 九、图标使用规范

### 9.1 Emoji 图标

- 用于导航项、卡片标题、功能区块标识
- 通过 `GlassCard` 的 `icon` prop 或模板直接插入
- 常用映射：

| 功能 | Emoji |
|------|-------|
| 仪表盘/数据 | `📊` `📈` |
| 设备/连接 | `📡` `⚡` |
| 运动/控制 | `🎯` |
| 测试/校准 | `🔧` `🔬` |
| 设置 | `⚙️` |
| 文件/存储 | `💾` `📁` `📂` |

### 9.2 Element Plus Icons

- 从 `@element-plus/icons-vue` 按需引入
- 用于按钮内部、操作图标、表单辅助图标
- 使用方式：`<el-icon><Setting /></el-icon>`

### 9.3 禁止

- 禁止混合使用多种图标库（如同时引入 FontAwesome、Remix 等）
- 禁止在状态指示器中使用 emoji 代替色点

---

## 十、文件与命名规范

### 10.1 样式文件组织

```
frontend/src/assets/styles/
├── variables.scss          # 唯一权威 SCSS token（已全局注入）
├── global.scss             # 全局 reset + Element Plus 覆盖
├── themes/
│   └── theme-variables.scss  # CSS 变量（多主题预留，当前不直接使用）
└── mixins.scss             # SCSS mixins（如需要）
```

### 10.2 Vue 组件样式

- **必须**使用 `<style lang="scss" scoped>`
- 属性顺序：`lang` 在前，`scoped` 在后（统一样式：`lang="scss" scoped`）
- 覆盖 Element Plus 内部样式必须使用 `:deep()`
- 禁止在 `<template>` 中使用 `style="..."` 内联样式

### 10.3 工具类规划

当前项目无 Tailwind/UnoCSS。如有频繁重复的 layout 模式，应抽象为 **全局 utility classes** 放入 `global.scss`，而非在每个组件中重写。

**候选工具类**（如需要可新增）：

```scss
// flex 工具
.flex-row { display: flex; align-items: center; }
.flex-col { display: flex; flex-direction: column; }
.flex-between { display: flex; justify-content: space-between; align-items: center; }
.gap-sm { gap: $spacing-sm; }
.gap-md { gap: $spacing-md; }
.gap-lg { gap: $spacing-lg; }

// 文本工具
.text-primary { color: $text-primary; }
.text-secondary { color: $text-secondary; }
.text-muted { color: $text-muted; }
.text-mono { font-family: $font-family-mono; }
.text-ellipsis { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
```

---

## 十一、反模式清单（禁止）

| # | 反模式 | 正确做法 |
|---|--------|----------|
| 1 | 在组件中写死 `#b829ff`、`#00f5ff` 等色值 | 使用 `$color-primary`、`$color-accent` |
| 2 | 手动复制 `GlassCard` 的样式代码到 view 中 | 直接使用 `<GlassCard>` 组件 |
| 3 | 混用 `variables.scss`、`theme-variables.scss`、`global.scss` 的色值 | 统一使用 `variables.scss` 的 SCSS 变量 |
| 4 | 在 template 中写 `style="margin-top: 12px"` | 使用 class + SCSS |
| 5 | 写 `padding: 13px`、`margin: 17px` 等非 token 值 | 使用 `$spacing-*` token |
| 6 | 在多个 view 中重复覆盖相同的 Element Plus 样式 | 提取到 `global.scss` |
| 7 | 状态指示器滥用 `animated` | 仅运行中/警告使用 pulse |
| 8 | 引入多个图标库 | 仅用 emoji + `@element-plus/icons-vue` |
| 9 | 组件 `<style>` 不写 `scoped` | 必须加 `scoped`，全局样式放入 `global.scss` |
| 10 | 使用 `StatusIndicator` 的 view 中同时手写 `.status-badge` | 统一使用 `StatusIndicator` |

---

## 十二、检查清单（Code Review 用）

提交前端代码前，确认以下各项：

- [ ] 所有颜色使用 SCSS 变量，无硬编码 hex
- [ ] 所有间距使用 `$spacing-*` token
- [ ] 卡片/面板优先使用 `GlassCard`
- [ ] 状态指示统一使用 `StatusIndicator`
- [ ] 数值展示统一使用 `ValueDisplay`
- [ ] 样式标签为 `<style lang="scss" scoped>`
- [ ] 无 template 内联 `style`
- [ ] Element Plus 覆盖通过 `:deep()` 且在正确层级
- [ ] 所有用户可见文本为中文
- [ ] 等宽字体用于数值显示

---

## 附录：Token 速查表

```scss
// 颜色
$color-primary: #b829ff;      $color-accent: #00f5ff;
$color-success: #00ff88;      $color-warning: #ffaa00;
$color-danger: #ff3366;       $color-info: #00aaff;

// 背景
$bg-primary: #0a0a1a;
$glass-bg: rgba(255,255,255,0.06);
$glass-bg-hover: rgba(255,255,255,0.10);
$glass-bg-elevated: rgba(255,255,255,0.08);

// 文字
$text-primary: #ffffff;
$text-secondary: rgba(255,255,255,0.8);
$text-tertiary: rgba(255,255,255,0.6);
$text-muted: rgba(255,255,255,0.4);

// 间距
$spacing-xs: 4px;   $spacing-sm: 8px;    $spacing-md: 12px;
$spacing-lg: 16px;  $spacing-xl: 24px;   $spacing-2xl: 32px;

// 圆角
$border-radius-sm: 8px;   $border-radius-md: 12px;
$border-radius-lg: 16px;  $border-radius-xl: 20px;

// 字体
$font-family-base: "Microsoft YaHei", ...;
$font-family-mono: "Microsoft YaHei", 'JetBrains Mono', ...;
$font-size-xs: 11px;  $font-size-sm: 13px;  $font-size-md: 14px;
$font-size-lg: 16px;  $font-size-xl: 18px;  $font-size-2xl: 20px;

// 过渡
$transition-fast: 150ms cubic-bezier(0.4, 0, 0.2, 1);
$transition-base: 250ms cubic-bezier(0.4, 0, 0.2, 1);
```
