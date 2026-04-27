# YX-DAQ 开发规范

> 从现有代码中提取的实际约定，所有新代码应遵循。

---

## 一、Go 后端规范

### 1.1 包结构

```
yx-daq/
├── main.go                    # 入口，//go:embed all:frontend/dist
├── app.go                     # Wails binding (~50 方法)，事件发布
└── internal/
    ├── types/                 # 共享类型定义，零依赖
    ├── driver/                # 硬件驱动层（TCP/UDP）
    ├── manager/               # 管理器层，定义 DeviceDriver/MotionController 接口
    ├── calibration/           # 五孔移位插值测试服务
    ├── three_hole/            # 三孔移位插值测试服务
    ├── storage/               # JSON 配置/CSV 录制/PDF 报告
    └── scanner/               # UDP 设备扫描
```

- `types` 包只定义类型，不包含构造方法。
- `manager` 包定义接口（`DeviceDriver`, `MotionController`），`driver` 包实现它们。
- `calibration` 和 `three_hole` 结构平行，模式相同。

### 1.2 import 分组

三个组，组间空行分隔：

```go
import (
    "context"
    "fmt"
    "time"

    "yx-daq/internal/manager"
    "yx-daq/internal/types"

    "github.com/wailsapp/wails/v2/pkg/runtime"
)
```

顺序：**stdlib → internal 包 → 第三方依赖**。别名只在包名冲突时使用（如 `wailsRuntime`）。

### 1.3 命名

| 元素 | 规则 | 示例 |
|------|------|------|
| 包名 | 小写，不要复数和下划线（`three_hole` 是目录名，包名与之匹配） | `package calibration` |
| 类型 | PascalCase | `type CalibrationService struct` |
| 导出函数 | PascalCase | `func NewCalibrationService(...)` |
| 非导出方法 | camelCase | `func (s *Service) runLoop()` |
| 变量 | camelCase | `deviceManager`, `acquisitionHub` |
| 接收器 | 单个字母 | `a *App`, `d *XYDAQDriver`, `s *Service` |
| 字符串枚举 | `type X string` + `const` 块 | `type DeviceType string` |
| JSON 标签 | lowerCamelCase，可选字段加 `omitempty` | `json:"currentPoint,omitempty"` |
| 函数类型 | `type XxxFunc func(...) (...)` | `type DataGetter func(deviceID string, channelIndex int) (float64, bool)` |

### 1.4 构造器模式

所有需要初始化的结构体都有 `NewXxx` 函数，返回指针：

```go
func NewCalibrationService(publisher EventPublisher) *CalibrationService {
    return &CalibrationService{
        eventPublisher: publisher,
        cancelCh:      make(chan struct{}),
        pauseCh:       make(chan struct{}),
        resumeCh:      make(chan struct{}),
    }
}
```

- 必要依赖：构造器注入
- 可选/循环依赖：setter 注入（如 `SetDataGetter`, `SetMotionController`）

### 1.5 错误处理

```go
// 必用 %w 包装
return fmt.Errorf("connect failed: %w", err)

// 非致命错误用 log.Printf
log.Printf("load config failed: %v", err)

// 不在非 main 包使用 println、panic、log.Fatal
```

### 1.6 并发

| 场景 | 工具 |
|------|------|
| 读写锁（map 保护） | `sync.RWMutex` |
| 复杂状态互斥 | `sync.Mutex` |
| 跨 goroutine 标志 | `sync/atomic.Bool` |
| 停止 goroutine | `chan struct{}` |
| 非阻塞停止发送 | `select { case ch <- struct{}{}: default: }` |
| 轮询 | `time.NewTicker` + `select` |

读操作加 `RLock`，写操作加 `Lock`。不要在持有锁时调用外部函数——先收集数据，释放锁，再处理。

### 1.7 接口定义

接口定义在使用它们的包中（通常是 `manager`），而不是在实现包中（`driver`）：

```go
// manager/device_manager.go
type DeviceDriver interface {
    Connect() error
    Disconnect()
    IsConnected() bool
    ...
}
```

hardware 驱动和模拟驱动都隐式实现这些接口，不需要显式声明 `implements`。

### 1.8 服务生命周期

服务统一使用四个方法：

```
Start(config) → Pause() → Resume() → Stop()
```

- `Start` 启动 goroutine，`Stop` 通过 `cancelCh` 通知退出
- `running` 用 `atomic.Bool`，状态用 `sync.Mutex` 保护的 struct
- goroutine 启动时 `defer running.Store(false)`
- 循环内通过 `select { case <-cancelCh: return }` 检查取消

### 1.9 注释

- 公开 API：中文
- 技术细节/算法注记：中文或英文均可，保持一致即可
- 段落分隔：`// ==================== 标题 ====================`

### 1.10 测试

- 文件放在被测试包内，包名相同（白盒测试）
- 表驱动测试 + `t.Run`
- 浮点比较用 `math.Abs(a-b) > 1e-6`
- 临时目录用 `t.TempDir()`
- 命名：`Test<Function>_<Scenario>`
- 运行：`go test ./internal/...`

---

## 二、前端 Vue/TS 规范

### 2.1 文件组织

```
frontend/src/
├── main.ts              # 入口：挂载 Vue/Pinia/Router/ElementPlus
├── App.vue              # 根组件，onMounted 启动 store 监听
├── api/enums.ts         # const 枚举 + 中文标签映射 + 工具函数
├── assets/styles/       # SCSS（variables.scss / global.scss / theme-variables.scss）
├── components/          # 通用组件
│   ├── GlassCard.vue    # 玻璃态卡片容器
│   ├── ChartPanel.vue   # ECharts 封装
│   ├── StatusIndicator.vue / ValueDisplay.vue / CalibPointEditor.vue
│   └── __tests__/       # 组件单元测试
├── layouts/
│   └── MainLayout.vue   # 侧边栏 + 顶栏 + <router-view>
├── stores/              # Pinia stores
│   ├── device.ts / motion.ts / calibration.ts / threeHoleTest.ts
└── views/               # 页面视图
    ├── DashboardView.vue / DeviceView.vue / MotionView.vue
    ├── ThreeHoleTestView.vue / DataView.vue / SettingsView.vue
    └── CalibrationView.vue  # 当前隐藏
```

### 2.2 所有 Vue 文件统一模板

```vue
<template>
  <!-- 只一个根元素 -->
</template>

<script setup lang="ts">
// import 顺序：Vue → Element Plus → Stores → Components → API
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useDeviceStore } from '../stores/device'
import GlassCard from '../components/GlassCard.vue'
</script>

<style lang="scss" scoped>
/* BEM-like class 命名 */
</style>
```

### 2.3 import 顺序

1. Vue/Pinia/Router 核心（`vue`, `pinia`, `vue-router`）
2. Element Plus 及其图标（`element-plus`, `@element-plus/icons-vue`）
3. Store（`../stores/`）
4. 组件（`../components/`）
5. API/枚举（`../api/`）
6. 样式（`./assets/`）—— 全局样式在 main.ts 引入

### 2.4 Wails 调用规范

**禁止**静态 import wailsjs。全部用动态 import：

```typescript
async function fetchProfiles() {
  try {
    const { GetDeviceProfiles } = await import('../../wailsjs/go/main/App')
    profiles.value = await GetDeviceProfiles() as DeviceProfile[]
  } catch (e: any) {
    console.warn('fetchProfiles failed:', e?.message || e)
  }
}
```

事件监听：

```typescript
function startListening() {
  import('../../wailsjs/runtime/runtime').then(({ EventsOn }) => {
    EventsOn('daq:data-snapshot', (data: DataPayload[]) => {
      snapshots.value = data
    })
  })
}
```

事件命名空间：`daq:*`, `motion:*`, `calibration:*`, `three-hole:*`

### 2.5 Pinia Store 规范

全部使用 setup 函数式（非 options 对象式）：

```typescript
export const useDeviceStore = defineStore('device', () => {
  // state — ref()
  const profiles = ref<DeviceProfile[]>([])
  // getters — computed()
  const isConnected = computed(() => statuses.value.some(s => s.status === 'Connected'))
  // actions — 普通函数
  async function fetchProfiles(): Promise<string | null> {
    try {
      const { GetDeviceProfiles } = await import('../../wailsjs/go/main/App')
      profiles.value = await GetDeviceProfiles() as DeviceProfile[]
      return null
    } catch (e: any) {
      return e?.message || String(e)
    }
  }
  function startListening() { ... }
  // 导出所有需要公开的状态和方法
  return { profiles, isConnected, fetchProfiles, startListening }
})
```

- 调用 Go 的 action 返回 `string | null`（null 成功，string 为错误消息），或 `{ success: boolean, error?: string }`
- Store 名：`useXxxStore`

### 2.6 组件 Props & Emits

```typescript
// Props — defineProps + withDefaults
const props = withDefaults(defineProps<{
  title?: string
  icon?: string
  elevated?: boolean
  value?: number
  precision?: number
  unit?: string
}>(), {
  elevated: false,
  precision: 3,
})

// Emits — defineEmits 泛型
const emit = defineEmits<{
  'update:modelValue': [points: CalibPoint[]]
  configure: [axisName: string]
}>()

// Expose — defineExpose
defineExpose({ open, getChart })
```

### 2.7 模板 Ref & 响应式

```typescript
// DOM ref
const chartRef = ref<HTMLDivElement>()
// 组件 ref（带类型）
const configDialog = ref<InstanceType<typeof AxisConfigDialog>>()

// ref vs shallowRef
// - 普通数据：ref()
// - 大对象/高频更新：shallowRef() + triggerRef()
// - ECharts 实例：shallowRef()
```

### 2.8 枚举模式

不用 TypeScript `enum`，用 `const` 对象 + 类型推导：

```typescript
export const DeviceType = {
  SIMULATED: 'SIMULATED',
  XY_DAQ8: 'XY-DAQ8',
  XY_DAQ16: 'XY-DAQ16',
} as const

export type DeviceTypeValue = typeof DeviceType[keyof typeof DeviceType]

export const DeviceTypeLabels: Record<DeviceTypeValue, string> = {
  [DeviceType.SIMULATED]: '模拟设备',
  [DeviceType.XY_DAQ8]: 'XY-DAQ8',
  [DeviceType.XY_DAQ16]: 'XY-DAQ16',
}
```

### 2.9 前端类型定义

类型定义在各 store/view 文件中就地声明，**不建 `frontend/src/types/` 目录**。接口名与 Go 后端结构体对应：

```typescript
interface ChannelConfig {
  name: string
  channelIndex: number
  unit: string
  // ...
}

interface DeviceProfile {
  id: string
  name: string
  type: DeviceTypeValue
  channels: ChannelConfig[]
  // ...
}
```

CSV 导出在 frontend 用 Blob，不在后端处理：

```typescript
function exportCSV() {
  const BOM = '\uFEFF'
  const csv = BOM + headers.join(',') + '\n' + rows.join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `export-${Date.now()}.csv`
  a.click()
  URL.revokeObjectURL(url)
}
```

### 2.10 Element Plus 规范

- Dialog：`v-model` 控制显隐，`append-to-body`，`#footer` 模板
- 消息提示：`ElMessage.success(...)` / `ElMessage.error(...)`
- 表格：`#default="{ row }"` 作用域插槽
- 图标：从 `@element-plus/icons-vue` import 具名导出后用在 `:icon="Xxx"`
- 按钮加载：`el-button :loading="xxxLoading"`

### 2.11 ECharts 规范

所有 ECharts 实例通过 `ChartPanel` 封装组件使用，不直接 `echarts.init`：

```html
<ChartPanel :option="chartOption" height="100%" />
```

chart option 作为 `computed` 对象，统一风格：

```typescript
const chartOption = computed(() => ({
  backgroundColor: 'transparent',
  tooltip: {
    backgroundColor: 'rgba(10,10,26,0.9)',
    borderColor: 'rgba(0,245,255,0.3)',
    textStyle: { color: '#fff' },
  },
  xAxis: {
    axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
    axisLabel: { color: 'rgba(255,255,255,0.4)' },
  },
  yAxis: {
    splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
  },
  series: [...],
}))
```

### 2.12 样式规范

- 所有组件使用 `<style lang="scss" scoped>`
- 全局样式在 `main.ts` 引入
- SCSS 变量自动注入：`@use "@/assets/styles/variables.scss" as *;`
- 深色霓虹主题，颜色规则：
  - 紫色 `#b829ff` — 主色 / Kalpha / X 轴
  - 青色 `#00f5ff` — 强调 / Kbeta / Y 轴
  - 绿色 `#00ff88` — 成功 / CPS / Z 轴
  - 橙色 `#ffaa00` — 警告 / CPT / U 轴
  - 红色 `#ff3366` — 危险 / 急停
- 路径别名：`@` → `/src`（vite.config.ts 配置）

### 2.13 测试规范

- Vitest + happy-dom（不是 jsdom）
- `mount()` 挂载组件
- 纯展示组件，不 mock store
- 测试文件位置：`src/components/__tests__/*.test.ts`
- 匹配模式：`src/**/*.{test,spec}.{js,ts}`
- 运行：`cd frontend && npm run test`

### 2.14 中文 UI

所有面向用户的字符串用中文（标签、按钮、消息提示、状态文字）。例外：Go 绑定方法名、事件名保持英文。

---

## 三、构建与工作流

### 3.1 命令速查

| 操作 | 命令 |
|------|------|
| 开发模式（热重载） | `wails dev` |
| 构建 exe | `build.bat`（先 `go build ./...` 再 `wails build`） |
| 构建 + NSIS 安装包 | `build.bat nsis` |
| 清理 | `build.bat clean` |
| Go 编译检查 | `go build ./...` |
| 前端类型检查 + 构建 | `cd frontend && npm run build` |
| 前端测试 | `cd frontend && npm run test` |
| Go 测试 | `go test ./internal/...` |

### 3.2 构建顺序

1. `//go:embed all:frontend/dist` 要求前端必须先构建
2. `build.bat` 自动先 `go build ./...` 检查编译，再 `wails build`
3. `npm run build` = `vue-tsc --noEmit && vite build`

### 3.3 配置存储

- 路径：`~/.yx-daq/`（用户 home 目录）
- 格式：JSON
- 写入方式：原子写入（`.tmp` → `Rename`）

---

## 四、重要约束

- **禁止**修改 `frontend/wailsjs/` 下自动生成的文件
- 前端构建产物 `frontend/dist/` 不提交到 git（在 `.gitignore` 中）
- `npm run build` 包含类型检查步骤，类型错误会阻塞构建
- 所有 Go 后端方法名导出后自动成为 Wails 前端绑定，注意命名不要冲突
- `CalibrationView` 路由存在但当前在侧边栏导航中隐藏
- 不加 `init()` 函数，用显式 `Init` 方法代替
