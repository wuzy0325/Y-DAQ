# YX-DAQ 编码规范

> 本文档从实际代码中提炼，覆盖 Go 后端和 Vue/TS 前端的编码约定。架构与目录结构见 `architecture.md`，性能规格见 `perf-spec.md`。新代码必须遵守。

---

## 一、Go 后端

### 1.1 Wails v3 服务绑定方法

本项目使用 Wails v3，绑定方法定义在 `internal/app/` 下的 `*Service` 结构体上（不是 v2 的 `*App`），按领域拆分到独立文件：

```
internal/app/
├── core.go               # Core struct + Startup/Shutdown + DI 汇聚
├── core_service.go       # CoreService → 绑定 Core 生命周期
├── service_device.go     # DeviceService → 设备管理绑定
├── service_motion.go     # MotionService → 运动控制绑定
├── service_three_hole.go # ThreeHoleService → 三孔测试绑定
├── service_five_hole.go  # FiveHoleService → 五孔测试绑定
├── service_calibration.go# CalibrationService → 五孔校准绑定
├── service_data.go       # DataService → 录制/回放绑定
├── service_config.go     # ConfigService → 配置/路径绑定
└── event_publishers.go   # 事件发布器实现
```

每个 `*Service` 持有 `Core` 指针，方法只做参数校验 + 调用 Core 上的业务服务 + 返回结果：

```go
type DeviceService struct {
    Core *core.Core
}

// 查询：返回 (结果, error) 或不返回 error
func (s *DeviceService) GetXxx() (Result, error) { ... }
func (s *DeviceService) GetXxx() Result { ... }

// 操作：返回 error；简单操作不返回 error
func (s *DeviceService) StartXxx(config Config) error { ... }
func (s *DeviceService) StopXxx() { ... }

// 文件对话框：返回 (路径, error)
func (s *DeviceService) SelectXxx() (string, error) { ... }
```

**规则**：
- 绑定方法只负责参数校验 + 调用业务服务 + 返回结果，**不含业务逻辑**
- nil check 放在第一行：`if s.Core.ConfigManager == nil { return err }`
- 错误直接向上抛出，不在绑定层吞没
- `Core` 是依赖注入汇聚点，不写业务逻辑；业务服务（`*TraversalService` 等）在 `internal/<domain>/` 包内
- 8 个 Service 在 `main.go` 中通过 `application.NewService(...)` 注册

### 1.2 服务生命周期

所有测试/采集服务统一实现 Start → Pause → Resume → Stop 生命周期：

```go
type Service struct {
    cancelCh  chan struct{}
    pauseCh   chan struct{}
    resumeCh  chan struct{}
    running   atomic.Bool
    mu        sync.Mutex
    status    Status
}

func (s *Service) Start(config Config) (string, error) {
    if s.running.Load() {
        return "", fmt.Errorf("already running")
    }
    s.running.Store(true)
    go s.runLoop(config)
    return taskID, nil
}

func (s *Service) Stop() {
    select {
    case s.cancelCh <- struct{}{}:
    default:
    }
}
```

**规则**：
- `running` 用 `atomic.Bool`，状态 struct 用 `sync.Mutex` 保护
- goroutine 退出时 `defer s.running.Store(false)`
- 循环内通过 `select { case <-s.cancelCh: return }` 检查取消
- `Stop()` 用非阻塞发送：`select { case ch <- struct{}{}: default: }`
- `Pause()` / `Resume()` 发送 pauseCh / resumeCh 信号

### 1.3 依赖注入模式

```go
// 必要依赖 —— 构造函数注入
func NewService(publisher EventPublisher) *Service {
    return &Service{eventPublisher: publisher, cancelCh: make(chan struct{})}
}

// 可选/循环依赖 —— Setter 注入（函数类型）
func (s *Service) SetBatchGetter(fn BatchGetter)     { s.batchGetter = fn }
func (s *Service) SetMotionController(fn MotionFunc)  { s.motionCtrl = fn }
func (s *Service) SetMotionWaiter(fn MotionWaitFunc)  { s.motionWaiter = fn }
```

**规则**：
- 函数类型用 `type XxxFunc func(...) (...)` 定义，放在 `types/` 或服务文件中
- Setter 在 `app.go` 的 `startup()` 中调用，用闭包连接各 manager
- manager 包不直接依赖 calibration/three_hole，通过回调注入解耦

### 1.4 ConfigStore 泛型

```go
// 定义（storage/config_store.go）
type ConfigStore[T any] struct {
    mu       sync.RWMutex
    filePath string
    data     T
}

func NewConfigStore[T any](filePath string, defaultData T) *ConfigStore[T] { ... }
func (s *ConfigStore[T]) Load() error   { ... }   // 文件→data，失败回退默认值
func (s *ConfigStore[T]) Get() T        { ... }   // 只读
func (s *ConfigStore[T]) Set(data T) error { ... } // 写入→原子保存

// 使用时指定具体类型
type ConfigManager struct {
    Devices   *ConfigStore[[]types.DeviceProfile]
    Motion    *ConfigStore[[]types.MotionControllerProfile]
    Storage   *ConfigStore[map[string]any]
    ThreeHole *ConfigStore[types.ThreeHoleTraversalConfig]
}
```

**规则**：
- 直接 `Get()` 返回强类型，**禁止** `json.Marshal` → `json.Unmarshal` 转换体操
- 写入使用原子操作：写 `.tmp` 文件 → `os.Rename` 替换原文件
- 配置路径统一在 `~/.yx-daq/` 下

### 1.5 错误处理

```go
// API 边界：%w 包装，保留错误链
return fmt.Errorf("connect failed: %w", err)

// 日志：结构化字段，避免 %v 拼接丢失上下文
slog.Error("load config failed", "err", err)

// 不允许的行为
// ❌ panic、log.Fatal（非 main 包）
// ❌ fmt.Errorf 中用 %v 包装 error（断链）
// ❌ 忽略 error 不处理也不注释原因
```

**规则**：
- 公开方法返回的 error 始终用 `%w` 包装底层 error
- 日志统一用 `log/slog`（`slog.Error` / `slog.Warn` / `slog.Info`），结构化字段而非 `%v` 拼接
- 忽略 error 必须写注释说明原因：`_ = conn.Close() // 关闭时连接可能已断开`

### 1.6 并发

| 场景 | 工具 |
|------|------|
| map 并发读写 | `sync.RWMutex` |
| 复杂状态互斥 | `sync.Mutex` |
| 跨 goroutine 标志 | `atomic.Bool` |
| goroutine 生命周期控制 | `context.Context`（统一规范，禁止裸 `chan struct{}`） |
| 轮询 | `time.NewTicker` + `select` |
| 回调字段（`onXxx`）读写 | 必须用所属结构的 `mu` 保护，禁止裸字段访问 |
| 单实例 goroutine 守护 | `atomic.Bool.CompareAndSwap`（参考 `MotionControllerManager.StartPolling`） |

> **强制规范（race detector 友好）**：
> 1. 所有回调字段（`onData` / `onSnapshot` / `onStatusChange` / `onReconnect` 等）的读写必须经过加锁的 setter / getter（如 `getOnData()`），禁止在 goroutine 中裸读 `b.onData`。参考 `TCPDriverBase`、`AcquisitionHub`、`MotionControllerManager` 的实现。
> 2. goroutine 取消统一用 `context.Context`，禁止用裸 `chan struct{}` 作为取消信号（`stopReconnect` 等历史代码除外，新代码必须用 ctx）。
> 3. 跨 goroutine 状态标志统一用 `atomic.Bool` / `atomic.Int64`，禁止裸 `bool` 字段跨 goroutine 共享。
> 4. 并发测试必须能在 `go test -race` 下通过，新增并发逻辑需补对应 `-race` 测试用例。

```go
// 锁的正确用法：不在持有锁时调用外部函数
func (m *Manager) pollStatus() {
    m.mu.RLock()
    toPoll := make(map[string]Controller)
    for id, ctrl := range m.instances {
        toPoll[id] = ctrl
    }
    m.mu.RUnlock()  // 先释放锁

    // 在锁外执行可能耗时的操作
    for id, ctrl := range toPoll {
        status, err := ctrl.GetStatus()
        ...
    }

    m.mu.Lock()  // 写结果时再获取写锁
    ...
    m.mu.Unlock()
}
```

```go
// 回调字段加锁模式：锁内取指针，锁外执行回调（避免回调内部再获取 mu 导致死锁）
func (b *Base) SetOnData(cb DataCallback) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.onData = cb
}

func (b *Base) getOnData() DataCallback {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return b.onData
}

func (b *Base) EmitData(payload DataPayload) {
    if cb := b.getOnData(); cb != nil {
        cb(payload)  // 回调在锁外执行
    }
}
```

### 1.7 事件发布

```go
// 事件发布器 —— 定义在 internal/app/event_publishers.go
type threeHoleEventPublisher struct {
    app *application.App
}

func (p *threeHoleEventPublisher) EmitProgress(event types.ThreeHoleTraversalProgressEvent) {
    p.app.Event.Emit("three-hole:progress", event)
}
```

**规则**：
- 事件命名：`<domain>:<action>`（如 `daq:data-snapshot`、`three-hole:progress`、`five-hole:realtime`）
- 发布器实现在 `internal/app/event_publishers.go`，持有 `*application.App` 引用以访问 `Event`
- 服务通过接口调用发布器，不直接依赖 Wails Runtime

### 1.8 接口定义

接口在使用方定义（通常是 manager 层），而非实现方（driver 层）：

```go
// manager/device_manager.go —— 定义接口
type DeviceDriver interface {
    Connect() error
    Disconnect()
    IsConnected() bool
    ...
}

// driver/simulated_device.go —— 隐式实现
type SimulatedDevice struct { ... }
func (d *SimulatedDevice) Connect() error { ... }
```

### 1.9 测试

```go
// 文件命名: <source>_test.go，包名与被测包相同（白盒测试）
// 函数命名: Test<Function>_<Scenario>
func TestCalculate_DeltaPZero(t *testing.T) { ... }

// 表驱动测试
tests := []struct {
    name string
    ...
}{...}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}

// 浮点比较
if math.Abs(result.X - expected.X) > 1e-6 { t.Errorf(...) }

// 临时文件
tmpDir := t.TempDir()
```

并发测试规范（race detector 友好）：
- 命名：`TestXxx_Concurrent_<Scenario>`，与功能测试区分
- 用 `sync.WaitGroup` 同步，用 `chan struct{}` 作为停止信号
- 运行时长用 `time.After` / `time.Sleep` 控制（建议 200ms-1s），不要用 sleep 做同步断言
- 必须在 `go test -race` 下通过；本地一键命令 `wails3 task test:race`（需 CGO_ENABLED=1 + gcc）
- CI 通过 `.github/workflows/go-test.yml` 自动执行 race 检测

### 1.10 健壮性（Robustness）

健壮性是本项目的核心质量目标，新代码必须满足以下条款：

**错误传播**：
- 公开方法返回的 error 始终用 `%w` 包装底层 error，保留错误链
- 收集多步操作的所有错误合并返回，而非仅返回首个错误（如运动控制器删除需清理多个资源）
- 忽略 error 必须写注释说明原因：`_ = conn.Close() // 关闭时连接可能已断开`

**并发安全**：
- `running` 状态用 `atomic.Bool`，复杂状态用 `sync.Mutex` / `sync.RWMutex`
- **持有锁时不调用外部函数**（死锁风险）：先拷贝出需要的数据，释放锁后再调用
- goroutine 退出时 `defer s.running.Store(false)`，长任务用 `sync.WaitGroup` 等待退出
- `Stop()` 用非阻塞发送：`select { case ch <- struct{}{}: default: }`
- map 字段读写必须经 mutex 保护，删除 key 后**不要**用零值判断存在性（用显式存在性检查）

**资源清理**：
- 启动新任务前清理上一次任务的残留状态（如 `completeProbeDataPoints = nil`）
- `defer` 用于关闭文件 / 连接 / 释放锁，按 LIFO 顺序
- 定时器 / 监听器在 `Stop()` 或组件卸载时显式清理（`clearTimeout(timer)`）
- 采样阶段长时间无新数据应自动暂停并发出 error 事件（停滞检测）

**失败恢复**：
- `panic` / `log.Fatal` 仅允许在 `main` 包；业务代码返回 error
- 配置加载失败回退默认值，不阻塞启动
- 配置写入用原子操作：写 `.tmp` → `os.Rename` 替换原文件
- 重复样本用 timestamp 去重，避免慢设备重复样本导致滤波失效

### 1.11 面向对象编码要点

本项目通过 Go 的组合 + 接口表达面向对象关键特征，编码层面遵循：

**封装**：
- struct 字段小写不导出，外部只能通过方法访问
- 状态字段与配置字段分组声明（依赖 → 状态 → 通道）
- 构造函数 `NewXxx()` 返回指针，必要依赖通过参数注入

**抽象与多态**：
- 接口在**使用方**定义（如 `manager` 定义 `DeviceDriver`，`driver` 隐式实现）
- 接口小而聚焦，1-5 个方法；不为每个实现单独建接口
- 真实驱动 / 模拟驱动可互换，测试中注入模拟实现

**组合优于继承**：
- 业务服务采用**编排者 + 协作者**模式：`Service` 持有 `TestManager` / `DataProcessor` / `RealtimeRecorder` / `MotionCoordinator` / `EventHandler` 等指针，自身只编排，不实现算法
- 协作者之间不直接互引，全部通过 `Service` 编排
- **禁止**嵌入带方法的 struct 模拟继承链

```go
// 好：编排者委托给单一职责协作者
func (s *FiveHoleTraversalService) StartTest(cfg Config) error {
    s.testManager.Start(cfg)          // 委托：生命周期
    s.pointGenerator.Generate(cfg)    // 委托：布点
    s.dataProcessor.Process(samples)  // 委托：数据处理
    return nil
}
```

---

## 二、前端 Vue/TypeScript

### 2.1 Pinia Store

全部使用 setup 函数式（Composition API 风格）：

```typescript
export const useXxxStore = defineStore('xxx', () => {
  // State —— ref / shallowRef
  const items = ref<Item[]>([])
  const isRunning = ref(false)

  // Getters —— computed
  const hasItems = computed(() => items.value.length > 0)

  // Actions —— 普通 async 函数
  async function fetchItems(): Promise<void> {
    try { items.value = await GetItems() as Item[] }
    catch (e) { console.warn('fetchItems failed:', e) }
  }

  function startListening() {
    EventsOn('domain:event', (data) => { ... })
  }

  return { items, isRunning, hasItems, fetchItems, startListening }
})
```

**规则**：
- Store 名：`useXxxStore`（Pinia 约定）
- 类型/接口在 store 文件顶部就地声明，不建单独 `types/` 目录
- `startListening()` 在 `App.vue` 的 `onMounted` 中调用
- Action 返回值约定：错误返回 `string`（错误消息），成功返回 `null`；复杂操作用 `{ success: boolean, error?: string }`

### 2.2 Wails API 调用

前端直接 import Wails v3 绑定：`@bindings/yx-daq/internal/app`（Service 对象）+ `@wailsio/runtime`（Events）。Store/View 均用静态 import。

```typescript
// Stores / Views —— 静态 import v3 绑定
import { DeviceService, DataService } from '@bindings/yx-daq/internal/app'
import { Events } from '@wailsio/runtime'
import * as types from '@bindings/yx-daq/internal/types'

// 调用 Service 方法
const profiles = await DeviceService.GetDeviceProfiles()

// 事件监听（回调参数 {data: T}，需 event.data 解包）
Events.On('daq:data-snapshot', (event) => {
  const data = event.data as DataPayload[]
  ...
})
Events.Off('daq:data-snapshot')
```

**规则**：
- Store / View 中**必须**静态 import Wails 绑定（`@bindings/...` + `@wailsio/runtime`）
- Wails types 仅在需要 `new types.Xxx(...)` 构造时导入
- **禁止**手动编辑 `src/frontend/bindings/`（自动生成）
- 新增绑定后运行 `cd src && wails3 generate bindings -clean=true -ts` 重新生成 `bindings/`

### 2.3 Composable

共享逻辑提取为 composable，放在 `src/frontend/src/composables/`：

```typescript
// composables/usePlayback.ts
export function usePlayback() {
  const playbackData = ref<PlaybackRow[]>([])
  const isPlaying = ref(false)
  ...

  function parseAndLoadCSV(content: string) { ... }
  const playbackProgress = computed(() => { ... })

  onUnmounted(() => { /* cleanup */ })

  return { playbackData, isPlaying, parseAndLoadCSV, playbackProgress, ... }
}
```

**规则**：
- 文件名：`use<Feature>.ts`，导出函数：`use<Feature>()`
- 涉及 timer/listener 的 composable 必须在 `onUnmounted` 中清理
- composable 不可直接引用 store，不可包含 UI 逻辑

### 2.4 Vue 组件

```vue
<template>
  <!-- 必须只有一个根元素 -->
  <div class="component-name">
    ...
  </div>
</template>

<script setup lang="ts">
// import 顺序: Vue → Element Plus → Store → Component → Composable → API
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useDeviceStore } from '../stores/device'
import GlassCard from '../components/GlassCard.vue'
import { usePlayback } from '../composables/usePlayback'
</script>

<style lang="scss" scoped>
/* BEM-like: .component-name__element--modifier */
.component-name { ... }
</style>
```

**规则**：
- 单根元素（方便样式隔离）
- 所有样式使用 `<style lang="scss" scoped>`
- Props 用 `defineProps` + `withDefaults` 泛型写法
- Emits 用 `defineEmits<{...}>()` 泛型写法
- Expose 用 `defineExpose({ ... })`

### 2.5 响应式

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

### 2.6 ECharts

```typescript
// Option 作为 computed 属性
const chartOption = computed(() => ({
  backgroundColor: 'transparent',
  tooltip: {
    trigger: 'axis',
    backgroundColor: 'rgba(10,10,26,0.9)',
    borderColor: 'rgba(0,245,255,0.3)',
    textStyle: { color: '#fff' },
  },
  grid: { left: 60, right: 20, top: 30, bottom: 30 },
  xAxis: {
    type: 'category',
    axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
    axisLabel: { color: 'rgba(255,255,255,0.4)', fontSize: 10 },
  },
  yAxis: {
    type: 'value',
    splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
  },
  series: [...],
}))
```

```html
<!-- 统一使用 ChartPanel 封装组件，不直接 echarts.init -->
<ChartPanel :option="chartOption" height="300px" />
```

**规则**：
- 所有图表通过 `<ChartPanel>` 组件渲染
- option 作为 `computed` 属性定义
- 颜色主题：青 `#00f5ff`（强调）、紫 `#b829ff`（主色）、绿 `#00ff88`（成功）、橙 `#ffaa00`（警告）、红 `#ff3366`（危险）

### 2.7 枚举

使用 `const` 对象 + 类型推导，**不用** TypeScript `enum`：

```typescript
export const TraversalPattern = {
  RECTANGLE: 'rectangle',
  LINE: 'line',
  CUSTOM: 'custom',
} as const

export type TraversalPatternValue = typeof TraversalPattern[keyof typeof TraversalPattern]

export const TraversalPatternLabels: Record<TraversalPatternValue, string> = {
  [TraversalPattern.RECTANGLE]: '矩形布点',
  [TraversalPattern.LINE]: '直线布点',
  [TraversalPattern.CUSTOM]: '自定义',
}
```

### 2.8 CSV 导出

```typescript
function exportCSV() {
  const BOM = '\uFEFF'  // Excel 正确识别 UTF-8
  const headers = ['列1', '列2', '列3']
  const rows = data.map(d => [d.field1, d.field2.toFixed(4), d.field3].join(','))
  const csv = BOM + headers.join(',') + '\n' + rows.join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `export-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.csv`
  a.click()
  URL.revokeObjectURL(url)
}
```

### 2.9 Element Plus

```html
<!-- Dialog -->
<el-dialog v-model="showDialog" title="标题" width="600px" :append-to-body="true">
  ...
  <template #footer>
    <el-button @click="showDialog = false">取消</el-button>
    <el-button type="primary" :loading="saving" @click="save">确定</el-button>
  </template>
</el-dialog>

<!-- Table with slot scope -->
<el-table :data="items">
  <el-table-column label="操作">
    <template #default="{ row }">
      <el-button @click="handle(row.id)">操作</el-button>
    </template>
  </el-table-column>
</el-table>
```

- 图标：从 `@element-plus/icons-vue` import 具名导出后用在 `:icon="Xxx"`
- 消息提示：`ElMessage.success(...)` / `ElMessage.error(...)`

### 2.10 样式

```scss
// 颜色规范（深色霓虹主题）
// 主色:      #b829ff (紫) —— Kalpha / X轴
// 强调:      #00f5ff (青) —— Kbeta / Y轴
// 成功:      #00ff88 (绿) —— CPS / Z轴 / 已连接
// 警告:      #ffaa00 (橙) —— CPT / U轴
// 危险:      #ff3366 (红) —— 急停 / 录制中 / error
// 半透明:    rgba(255,255,255,0.05~0.9) —— 背景层级
```

**规则**：
- SCSS 变量在 `assets/styles/variables.scss` 中定义，Vite 自动注入所有 SCSS
- 路径别名：`@` → `src/frontend/src/`（`src/frontend/vite.config.ts` 配置）

### 2.11 测试

```typescript
// Vitest + happy-dom
// 文件位置: src/components/__tests__/<Component>.test.ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ComponentName from '../ComponentName.vue'

describe('ComponentName', () => {
  it('renders correctly', () => {
    const wrapper = mount(ComponentName, { props: { ... } })
    expect(wrapper.text()).toContain('...')
  })
})
```

- 纯展示组件，不 mock store
- 匹配模式：`src/**/*.{test,spec}.{js,ts}`
- 运行：`cd src/frontend && npm run test`

### 2.12 中文 UI

所有面向用户的字符串用中文（标签、按钮、消息提示、状态文字）。例外：Go 绑定方法名、事件名保持英文。

### 2.13 健壮性与面向对象要点（前端）

**健壮性**：
- 涉及 timer / 事件监听的组件或 composable，必须在 `onUnmounted` 中显式清理（`clearTimeout` / `EventsOff`），避免残留回调触发后端监控重启
- Store action 错误返回 `string`（错误消息）或 `null`（成功）；复杂操作用 `{ success, error? }`，不抛未捕获异常
- 启动新测试前清理上一次测试的缓存数据（如 `completeProbeDataPoints.value = null`），防止旧数据残留导致导出错误
- 大对象 / 高频更新用 `shallowRef()` + `triggerRef()`，避免深层响应式开销
- 枚举值用 `const` 对象 + 类型推导，**禁止**手工拼接字符串（避免枚举值变更时的隐式错误）

**面向对象（组合优于继承）**：
- Vue 组件通过 `composables/` 复用逻辑，**禁止** `extends` / `mixins`
- Props + Emits 是组件间通信标准方式，不用 `provide/inject` 传递业务数据
- Store 内部状态封装为 `ref`，仅通过暴露的 action 方法修改，外部不直接写 store 字段

---

## 三、跨域规范

### 3.1 Import 顺序

**Go**：
```
stdlib ("context", "fmt", ...)

internal 包 ("yx-daq/internal/...")

第三方 ("github.com/wailsapp/...")
```

**TypeScript/Vue**：
```
Vue 核心 (vue, pinia, vue-router)

Element Plus (element-plus, @element-plus/icons-vue)

Store (../stores/)

Component (../components/)

Composable/API (../composables/, ../api/)

Wails 绑定 (@bindings/yx-daq/internal/app)  ← stores/views 静态导入
```

### 3.2 命名

| 类别 | 规则 | 示例 |
|------|------|------|
| Go 包名 | 全小写，无下划线（目录名可与包名不同） | `package calibration` |
| Go 导出类型 | PascalCase | `ThreeHoleTraversalService` |
| Go 非导出方法 | camelCase | `func (s *Service) runLoop()` |
| Go 接收器 | 单字母 | `c *Core`, `s *Service`, `m *Manager` |
| Go 接口 | `-er` 后缀或明确动词 | `EventPublisher`, `BatchGetter` |
| Go 文件名 | `snake_case.go` | `config_store.go` |
| Go JSON 标签 | lowerCamelCase，可选字段加 `omitempty`；领域缩写（Kalpha/Kbeta/CPT/CPS）保留 PascalCase | `json:\"currentPoint,omitempty\"` |
| Go 函数类型 | `type XxxFunc func(...) (...)` | `type DataGetter func(deviceID string, channelIndex int) (float64, bool)` |
| Vue 组件文件 | PascalCase | `GlassCard.vue` |
| Vue 视图文件 | PascalCase + `View` 后缀 | `DeviceView.vue` |
| Store 文件 | camelCase | `device.ts`, `threeHoleTest.ts` |
| TS 接口/类型 | PascalCase | `DeviceProfile`, `PlaybackRow` |
| TS 变量/函数 | camelCase | `dataSavePath`, `fetchProfiles()` |
| TS 枚举 | PascalCase `const` 对象 | `TraversalPattern` |
| 目录名 | PascalCase（组件目录）/ camelCase（非组件） | `MotionControl/`, `stores/` |

### 3.3 注释

```go
// ==================== 标题分隔 ====================

// 公开函数：中文简述功能
// 私有实现：不写注释（函数名自解释）
```

```typescript
// ==================== 区域标识 ====================

// 单行注释：解释 WHY（非 WHAT）
```

**规则**：
- Go 公开方法/函数写中文注释
- 代码自解释时不加注释
- 分隔符：`// ==================== 标题 ====================`
- 不写"修改记录"、"作者"、"日期"注释（用 git 查）

### 3.4 禁止事项

| 反模式 | 原因 | 正确做法 |
|--------|------|---------|
| `interface{}` 在 ConfigStore 中 | 需要来回 JSON 转换 | 泛型 `ConfigStore[T]` |
| `json.Marshal` → `json.Unmarshal` 转类型 | 性能浪费，类型不安全 | 直接 `Get()` 返回强类型 |
| `fmt.Errorf("...: %v", err)` | 断错误链 | 用 `%w` |
| Go 文件超过 500 行 | 难以维护 | 拆分子文件 |
| 前端 store 中动态 import Wails 绑定 | 每次调用都重复解析 | 静态 import |
| 在 `components/` 中引用 `views/` | 违反分层 | views 引用 components |
| 手动编辑 `src/frontend/bindings/` | 自动生成，下次构建覆盖 | 通过 `wails3 generate bindings` 重新生成 |
| `internal/utils/` 通用工具包 | 职责不清，垃圾桶 | 按业务领域拆分 |
| 在持有锁时调用外部函数 | 死锁风险 | 先释放锁再调用 |
| `log.Fatal` / `panic` 在非 main 包 | 无法优雅恢复 | 返回 error |

---

## 四、构建与工作流

### 4.1 命令速查

> 所有 `wails3`/`go` 命令在 `src/` 目录运行（Wails v3 工程根 + Go 模块根）。

| 操作 | 命令 |
|------|------|
| 开发模式（热重载） | `cd src && wails3 dev` |
| 构建 exe | `cd src && wails3 task build` |
| 构建 + NSIS 安装包 | `cd src && wails3 task package` |
| 清理 | `rmdir /s /q bin && rmdir /s /q src\frontend\dist` |
| Go 编译检查 | `cd src && go build ./...` |
| 前端类型检查 + 构建 | `cd src/frontend && npm run build` |
| 前端测试 | `cd src/frontend && npm run test` |
| Go 测试 | `cd src && go test ./internal/...` |

### 4.2 构建顺序

1. `//go:embed all:frontend/dist`（在 `src/main.go`）要求前端必须先构建
2. `cd src && wails3 task build` 调用 Wails v3 Taskfile 构建前端、生成资源并执行 Go 编译
3. `npm run build` = `vue-tsc --noEmit && vite build`
4. 构建产物输出到仓库根 `bin/`（Taskfile 中 `BIN_DIR="../bin"`）

### 4.3 配置存储

- 路径：`~/.yx-daq/`（用户 home 目录）
- 格式：JSON
- 写入方式：原子写入（`.tmp` → `Rename`）

---

## 五、重要约束

- **禁止**修改 `src/frontend/bindings/`（Wails v3 自动生成）
- 前端直接 import `@bindings/yx-daq/internal/app`（Service 对象）+ `@wailsio/runtime`（Events）访问 Wails 绑定
- 前端构建产物 `src/frontend/dist/` 不提交到 git（在 `.gitignore` 中）
- `npm run build` 包含类型检查步骤，类型错误会阻塞构建
- 所有 Go 后端方法名导出后自动成为 Wails 前端绑定，注意命名不要冲突
- `CalibrationView` 路由存在但当前在侧边栏导航中隐藏
- 不加 `init()` 函数，用显式 `Init` 方法代替

---

## 六、新功能 Checklist

开发新功能时按此清单逐项检查：

### Go 后端
- [ ] 文件放在正确的 `internal/` 子包（按业务领域，不按技术层）
- [ ] 公开方法/类型写中文注释
- [ ] 错误包装用 `%w`；多步操作收集所有错误合并返回
- [ ] 服务使用 Start/Pause/Resume/Stop 生命周期
- [ ] 必要依赖构造注入，可选依赖 Setter 注入
- [ ] 业务服务采用编排者 + 协作者模式，自身不实现算法
- [ ] 跨包调用通过接口，不依赖具体实现
- [ ] 并发安全：锁 / atomic / channel 选择正确；持有锁时不调用外部函数
- [ ] 启动新任务前清理上一次任务残留状态；defer 关闭资源
- [ ] 不引入循环依赖
- [ ] 测试与被测文件同包

### 前端
- [ ] Store/View 中 Wails 绑定使用**静态** import `@bindings/yx-daq/internal/app` + `@wailsio/runtime`
- [ ] 事件监听在 `startListening()` 中注册，`onUnmounted` 中清理
- [ ] 所有 UI 字符串用中文
- [ ] 枚举用 `const` 对象不用 `enum`，不用手工拼接字符串
- [ ] 图表通过 `ChartPanel` 组件渲染
- [ ] 样式使用 `<style lang="scss" scoped>`
- [ ] composable 不引用 store，不含 UI 逻辑
- [ ] 组件 Props 用 `defineProps` + `withDefaults` 泛型写法
- [ ] 启动新测试前清理上一次缓存数据

### 构建
- [ ] `go build ./...` 通过
- [ ] `cd frontend && npm run build` 通过
- [ ] `go test ./internal/...` 通过
- [ ] `cd frontend && npm run test` 通过
