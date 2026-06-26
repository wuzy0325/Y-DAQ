# YX-DAQ-T 热电偶类型设置功能 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 YX-DAQ-T 设备增加热电偶类型设置功能，移除不适用的压力单位选项，支持全通道统一设置和各通道独立设置，设备连接后主动读取热电偶类型并更新配置。

**Architecture:** 后端在 `ChannelConfig` 增加 `thermocoupleType` 字段，通过 `DeviceManager` 新增 `SetThermocoupleType` 方法暴露给前端；驱动层已有 `SetThermocoupleType` 和 `@e3` 读取命令，需在连接回调中自动读取并更新通道配置；前端编辑对话框对 YX-DAQ-T 设备隐藏"压力单位"，增加热电偶类型选择器（统一+独立），通道表格增加热电偶类型列。

**Tech Stack:** Go 1.23 / Wails v3 / Vue 3 + TypeScript / Element Plus

---

## 文件结构

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `internal/types/device.go` | `ChannelConfig` 增加 `ThermocoupleType` 字段 |
| 修改 | `internal/driver/yx_daqt_config.go` | `SetThermocoupleType` 支持单通道设置；`syncHardwareConfig` 回调更新通道配置 |
| 修改 | `internal/driver/yx_daqt.go` | 导出 `GetHardwareConfig` 为公共方法（已存在） |
| 修改 | `internal/manager/device_manager.go` | 新增 `ThermocoupleTypeSetter` 接口 + `SetThermocoupleType` 方法；`Connect` 中注册 `onConfigSynced` 回调 |
| 修改 | `internal/app/service_device.go` | 新增 `SetThermocoupleType` 和 `GetThermocoupleTypes` Wails 服务方法 |
| 修改 | `frontend/src/api/enums.ts` | 新增热电偶类型常量和选项 |
| 修改 | `frontend/src/stores/device.ts` | 新增 `setThermocoupleType` 方法；`ChannelConfig` 接口增加 `thermocoupleType` |
| 修改 | `frontend/src/views/DeviceView.vue` | YX-DAQ-T 隐藏"压力单位"；增加热电偶类型选择器；通道表格增加热电偶类型列 |
| 修改 | `frontend/wailsjs/go/main/App.js` | Wails 自动生成（运行 `wails3 generate bindings`） |
| 修改 | `frontend/wailsjs/go/main/App.d.ts` | Wails 自动生成（运行 `wails3 generate bindings`） |

---

### Task 1: ChannelConfig 增加 ThermocoupleType 字段

**Files:**
- Modify: `internal/types/device.go:107-115`

- [ ] **Step 1: 在 ChannelConfig 结构体中添加 ThermocoupleType 字段**

在 `internal/types/device.go` 的 `ChannelConfig` 结构体中添加 `ThermocoupleType` 字段：

```go
// ChannelConfig 通道配置
type ChannelConfig struct {
	Index            int     `json:"index"`
	Name             string  `json:"name"`
	Enabled          bool    `json:"enabled"`
	Unit             string  `json:"unit"`
	Precision        int     `json:"precision"`
	RangeMin         float64 `json:"rangeMin"`
	RangeMax         float64 `json:"rangeMax"`
	ThermocoupleType string  `json:"thermocoupleType,omitempty"` // 热电偶类型（K/J/T/E/N/S/R/B），仅 YX-DAQ-T
}
```

- [ ] **Step 2: 验证 Go 编译通过**

Run: `cd yx-daq && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 3: Commit**

```bash
git add yx-daq/internal/types/device.go
git commit -m "feat: add ThermocoupleType field to ChannelConfig"
```

---

### Task 2: 驱动层 — 支持单通道热电偶类型设置 + 连接后自动读取更新

**Files:**
- Modify: `internal/driver/yx_daqt_config.go`
- Modify: `internal/driver/yx_daqt.go`

- [ ] **Step 1: 修改 SetThermocoupleType 支持单通道设置**

在 `internal/driver/yx_daqt_config.go` 中，保留现有 `SetThermocoupleType` 方法（全通道批量设置），新增 `SetSingleThermocoupleType` 方法：

```go
// SetSingleThermocoupleType 设置单个通道的热电偶类型
// channelIndex: 0-15, tcType: 热电偶类型字符（K/J/T/E/N/S/R/B）
func (d *YXDAQTDriver) SetSingleThermocoupleType(channelIndex int, tcType string) error {
	if channelIndex < 0 || channelIndex > 15 {
		return fmt.Errorf("channel index must be 0-15, got %d", channelIndex)
	}
	validTypes := map[string]bool{"K": true, "J": true, "T": true, "E": true, "N": true, "S": true, "R": true, "B": true}
	if !validTypes[tcType] {
		return fmt.Errorf("unsupported thermocouple type: %s (supported: K, J, T, E, N, S, R, B)", tcType)
	}

	// 读取当前所有通道的热电偶类型
	current := d.hwConfig.ThermocoupleTypes
	if len(current) != 16 {
		current = "KKKKKKKKKKKKKKKK" // 默认全 K
	}

	// 修改指定通道
	runes := []rune(current)
	runes[channelIndex] = []rune(tcType)[0]
	newTypes := string(runes)

	// 发送完整的热电偶类型命令
	return d.SetThermocoupleType(newTypes)
}
```

- [ ] **Step 2: 在 syncHardwareConfig 中通过回调更新通道配置**

修改 `internal/driver/yx_daqt_config.go` 的 `syncHardwareConfig` 方法，在 `onConfigSynced` 回调前，将读取到的热电偶类型更新到 `d.channels`：

在 `d.hwConfig = config` 之后、`if d.onConfigSynced != nil` 之前，添加：

```go
	// 将热电偶类型同步到通道配置
	if len(config.ThermocoupleTypes) == 16 {
		d.mu.Lock()
		for i := range d.channels {
			if i < 16 {
				d.channels[i].ThermocoupleType = string(config.ThermocoupleTypes[i])
			}
		}
		d.mu.Unlock()
	}
```

- [ ] **Step 3: 验证 Go 编译通过**

Run: `cd yx-daq && go build ./...`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add yx-daq/internal/driver/yx_daqt_config.go yx-daq/internal/driver/yx_daqt.go
git commit -m "feat: add single-channel thermocouple type setting and auto-sync on connect"
```

---

### Task 3: Manager 层 — ThermocoupleTypeSetter 接口 + SetThermocoupleType 方法 + 连接回调

**Files:**
- Modify: `internal/manager/device_manager.go`

- [ ] **Step 1: 新增 ThermocoupleTypeSetter 接口和 SetThermocoupleType 方法**

在 `internal/manager/device_manager.go` 中，在 `UnitSetter` 接口之后添加：

```go
// ThermocoupleTypeSetter 热电偶类型设置接口（仅 YX-DAQ-T 驱动实现）
type ThermocoupleTypeSetter interface {
	SetThermocoupleType(tcTypes string) error
	SetSingleThermocoupleType(channelIndex int, tcType string) error
}

// SetThermocoupleType 设置设备热电偶类型（全通道批量设置，写入硬件）
func (m *DeviceManager) SetThermocoupleType(id string, tcTypes string) error {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}

	setter, ok := drv.(ThermocoupleTypeSetter)
	if !ok {
		return fmt.Errorf("device does not support SetThermocoupleType: %s", id)
	}

	if err := setter.SetThermocoupleType(tcTypes); err != nil {
		return err
	}

	// 更新 profile 中的通道热电偶类型
	m.mu.Lock()
	if profile, exists := m.profiles[id]; exists {
		if len(tcTypes) == 16 {
			for i := range profile.Channels {
				if i < 16 {
					profile.Channels[i].ThermocoupleType = string(tcTypes[i])
				}
			}
		}
		m.profiles[id] = profile
	}
	m.mu.Unlock()
	m.saveProfiles()

	return nil
}

// SetSingleThermocoupleType 设置单个通道的热电偶类型（写入硬件）
func (m *DeviceManager) SetSingleThermocoupleType(id string, channelIndex int, tcType string) error {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}

	setter, ok := drv.(ThermocoupleTypeSetter)
	if !ok {
		return fmt.Errorf("device does not support SetThermocoupleType: %s", id)
	}

	if err := setter.SetSingleThermocoupleType(channelIndex, tcType); err != nil {
		return err
	}

	// 更新 profile 中对应通道的热电偶类型
	m.mu.Lock()
	if profile, exists := m.profiles[id]; exists {
		for i := range profile.Channels {
			if profile.Channels[i].Index == channelIndex {
				profile.Channels[i].ThermocoupleType = tcType
				break
			}
		}
		m.profiles[id] = profile
	}
	m.mu.Unlock()
	m.saveProfiles()

	return nil
}
```

- [ ] **Step 2: 在 Connect 方法中注册 onConfigSynced 回调**

在 `internal/manager/device_manager.go` 的 `Connect` 方法中，在 `drv.SetDataCallback(...)` 之后、`drv.Connect()` 之前，添加 `onConfigSynced` 回调注册：

```go
	// DAQ-T 设备：注册配置同步回调，连接后自动读取热电偶类型并更新 profile
	if tcSetter, ok := drv.(ThermocoupleTypeSetter); ok {
		if notifier, ok := drv.(interface{ OnConfigSynced(func(interface{})) }); ok {
			notifier.OnConfigSynced(func(_ interface{}) {
				if hwGetter, ok := drv.(interface{ GetHardwareConfig() interface{} }); ok {
					_ = hwGetter // 配置已通过驱动内部 syncHardwareConfig 更新到 channels
				}
				// 从驱动获取更新后的通道配置并同步到 profile
				if channelGetter, ok := drv.(interface{ GetChannels() []types.ChannelConfig }); ok {
					updatedChannels := channelGetter.GetChannels()
					m.mu.Lock()
					if profile, exists := m.profiles[id]; exists {
						profile.Channels = updatedChannels
						m.profiles[id] = profile
					}
					m.mu.Unlock()
					m.saveProfiles()
					m.emitStatusChange()
				}
			})
		}
	}
```

注意：由于 `OnConfigSynced` 的回调签名是 `func(DAQTHardwareConfig)`，而 `DeviceDriver` 接口不暴露此方法，需要用类型断言。但更简洁的做法是直接在 `newYXDAQTDriver` 工厂函数中注册回调。修改 `newYXDAQTDriver`：

```go
func newYXDAQTDriver(profile types.DeviceProfile) DeviceDriver {
	drv := driver.NewYXDAQTDriver(profile.Host, profile.Port, profile.Channels)
	return drv
}
```

改为在 `DeviceManager.Connect` 中注册回调。由于 `YXDAQTDriver` 是具体类型，不在 `manager` 包的导入范围内（已通过 `driver` 包导入），我们可以利用 `DeviceDriver` 接口的 `GetChannels()` 方法来获取更新后的通道配置。

更优方案：在 `Connect` 方法中，连接成功后（`drv.Connect()` 返回 nil 后），检查驱动是否支持热电偶类型，如果是则等待配置同步完成并更新 profile。但这会阻塞 `Connect` 调用。

**最终方案**：在 `Connect` 方法中，连接成功后，利用已有的 `onConfigSynced` 回调机制。在 `drv.SetDataCallback(...)` 之后添加：

```go
	// DAQ-T 设备：注册配置同步回调，连接后自动读取热电偶类型并更新 profile
	if tcDrv, ok := drv.(*driver.YXDAQTDriver); ok {
		tcDrv.OnConfigSynced(func(_ driver.DAQTHardwareConfig) {
			updatedChannels := tcDrv.GetChannels()
			m.mu.Lock()
			if profile, exists := m.profiles[id]; exists {
				profile.Channels = updatedChannels
				m.profiles[id] = profile
			}
			m.mu.Unlock()
			m.saveProfiles()
			m.emitStatusChange()
		})
	}
```

这需要在 `device_manager.go` 中导入 `yx-daq/internal/driver` 包（已经导入了）。

- [ ] **Step 3: 验证 Go 编译通过**

Run: `cd yx-daq && go build ./...`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add yx-daq/internal/manager/device_manager.go
git commit -m "feat: add ThermocoupleTypeSetter interface and auto-sync on connect"
```

---

### Task 4: Service 层 — 新增 SetThermocoupleType 和 GetThermocoupleTypes Wails 服务方法

**Files:**
- Modify: `internal/app/service_device.go`

- [ ] **Step 1: 新增 SetThermocoupleType 方法**

在 `internal/app/service_device.go` 的 `SetUnit` 方法之后添加：

```go
// SetThermocoupleType 设置设备热电偶类型（全通道批量设置）
func (s *DeviceService) SetThermocoupleType(id string, tcTypes string) error {
	if s.Core.DeviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	return s.Core.DeviceManager.SetThermocoupleType(id, tcTypes)
}

// SetSingleThermocoupleType 设置单个通道的热电偶类型
func (s *DeviceService) SetSingleThermocoupleType(id string, channelIndex int, tcType string) error {
	if s.Core.DeviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	return s.Core.DeviceManager.SetSingleThermocoupleType(id, channelIndex, tcType)
}
```

- [ ] **Step 2: 验证 Go 编译通过**

Run: `cd yx-daq && go build ./...`
Expected: 编译成功

- [ ] **Step 3: 重新生成 Wails 绑定**

Run: `cd yx-daq && wails3 generate bindings -clean=true -ts`
Expected: 生成成功，`frontend/wailsjs/go/main/App.js` 和 `App.d.ts` 中新增 `SetThermocoupleType` 和 `SetSingleThermocoupleType`

- [ ] **Step 4: Commit**

```bash
git add yx-daq/internal/app/service_device.go yx-daq/frontend/wailsjs/
git commit -m "feat: add SetThermocoupleType and SetSingleThermocoupleType to DeviceService"
```

---

### Task 5: 前端 — 热电偶类型枚举和 Store 方法

**Files:**
- Modify: `frontend/src/api/enums.ts`
- Modify: `frontend/src/stores/device.ts`

- [ ] **Step 1: 在 enums.ts 中新增热电偶类型常量**

在 `frontend/src/api/enums.ts` 末尾添加：

```typescript
// 热电偶类型
export const ThermocoupleType = {
  K: 'K',
  J: 'J',
  T: 'T',
  E: 'E',
  N: 'N',
  S: 'S',
  R: 'R',
  B: 'B',
} as const

export type ThermocoupleTypeValue = typeof ThermocoupleType[keyof typeof ThermocoupleType]

// 热电偶类型中文标签
export const ThermocoupleTypeLabels: Record<ThermocoupleTypeValue, string> = {
  [ThermocoupleType.K]: 'K 型',
  [ThermocoupleType.J]: 'J 型',
  [ThermocoupleType.T]: 'T 型',
  [ThermocoupleType.E]: 'E 型',
  [ThermocoupleType.N]: 'N 型',
  [ThermocoupleType.S]: 'S 型',
  [ThermocoupleType.R]: 'R 型',
  [ThermocoupleType.B]: 'B 型',
}

// 热电偶类型选项（用于 el-select）
export const thermocoupleTypeOptions: { value: ThermocoupleTypeValue; label: string }[] =
  Object.entries(ThermocoupleTypeLabels).map(([value, label]) => ({
    value: value as ThermocoupleTypeValue,
    label,
  }))
```

- [ ] **Step 2: 在 device.ts 的 ChannelConfig 接口中增加 thermocoupleType 字段**

修改 `frontend/src/stores/device.ts` 的 `ChannelConfig` 接口：

```typescript
interface ChannelConfig {
  index: number
  name: string
  enabled: boolean
  unit: string
  precision: number
  rangeMin: number
  rangeMax: number
  thermocoupleType?: string
}
```

- [ ] **Step 3: 在 device.ts 中新增 setThermocoupleType 和 setSingleThermocoupleType 方法**

在 `frontend/src/stores/device.ts` 的 import 中添加新的 Wails 绑定：

```typescript
import {
  GetDeviceProfiles, UpdateDeviceProfile,
  ConnectDevice, DisconnectDevice,
  StartAcquisition, StopAcquisition,
  StartAcquisitionAll, StopAcquisitionAll,
  GetDeviceStatusAll, ScanDevices,
  SetUnit, SetThermocoupleType, SetSingleThermocoupleType,
} from '../../wailsjs/go/main/App'
```

在 store 中 `setUnit` 方法之后添加：

```typescript
  async function setThermocoupleType(id: string, tcTypes: string): Promise<string | null> {
    try {
      await SetThermocoupleType(id, tcTypes)
      await fetchProfiles()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('setThermocoupleType failed:', msg)
      return msg
    }
  }

  async function setSingleThermocoupleType(id: string, channelIndex: number, tcType: string): Promise<string | null> {
    try {
      await SetSingleThermocoupleType(id, channelIndex, tcType)
      await fetchProfiles()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('setSingleThermocoupleType failed:', msg)
      return msg
    }
  }
```

在 return 对象中添加 `setThermocoupleType` 和 `setSingleThermocoupleType`。

- [ ] **Step 4: 验证前端类型检查通过**

Run: `cd yx-daq/frontend && npx vue-tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 5: Commit**

```bash
git add yx-daq/frontend/src/api/enums.ts yx-daq/frontend/src/stores/device.ts
git commit -m "feat: add thermocouple type enums and store methods"
```

---

### Task 6: 前端 — DeviceView 编辑对话框改造

**Files:**
- Modify: `frontend/src/views/DeviceView.vue`

- [ ] **Step 1: 导入热电偶类型枚举**

在 `<script setup>` 的 import 区域添加：

```typescript
import { thermocoupleTypeOptions, ThermocoupleTypeLabels } from '../api/enums'
import type { ThermocoupleTypeValue } from '../api/enums'
```

- [ ] **Step 2: 修改 EditChannel 接口，增加 thermocoupleType 字段**

```typescript
interface EditChannel {
  index: number
  name: string
  enabled: boolean
  unit: string
  precision: number
  rangeMin: number
  rangeMax: number
  thermocoupleType: string
}
```

- [ ] **Step 3: 修改"通道参数"区域 — YX-DAQ-T 隐藏压力单位，增加热电偶类型选择器**

将现有的"通道参数"区域（约 L163-L183）替换为：

```html
      <div class="dialog-section">
        <div class="section-title">⚙️ 通道参数</div>
        <div class="form-row">
          <!-- 非温度设备：显示压力单位 -->
          <div v-if="editProfileType !== 'YX-DAQ-T'" class="form-group">
            <label class="group-label">压力单位</label>
            <el-select v-model="editForm.unit" filterable allow-create size="small" style="width: 100px">
              <el-option v-for="u in unitOptions" :key="u" :label="u" :value="u" />
            </el-select>
            <span class="hint-text">CH1-CH{{ editPressureCount }}</span>
          </div>
          <!-- 温度设备：显示热电偶类型（统一设置） -->
          <div v-else class="form-group">
            <label class="group-label">热电偶类型</label>
            <el-select v-model="editForm.thermocoupleType" size="small" style="width: 100px" @change="syncThermocoupleTypeToChannels">
              <el-option v-for="opt in thermocoupleTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="hint-text">统一设置所有通道</span>
          </div>
          <div class="form-group">
            <label class="group-label">精度</label>
            <el-input-number v-model="editForm.precision" :min="0" :max="6" size="small" style="width: 70px" controls-position="right" />
            <span class="hint-text">所有通道</span>
          </div>
          <div class="form-group">
            <label class="group-label">特殊通道</label>
            <span class="special-channels" v-if="editProfileType !== 'YX-DAQ-T'">CH{{ editPressureCount + 1 }}: 大气压 | CH{{ editPressureCount + 2 }}: 大气温度</span>
            <span class="special-channels" v-else>16 通道热电偶温度</span>
          </div>
        </div>
      </div>
```

- [ ] **Step 4: 在 editForm 中增加 thermocoupleType 字段**

修改 `editForm` 的 ref 初始值：

```typescript
const editForm = ref({
  id: '',
  name: '',
  host: '',
  port: 9000,
  publishRate: 20,
  unit: 'kPa',
  precision: 3,
  autoConnect: true,
  thermocoupleType: 'K',
})
```

- [ ] **Step 5: 修改 openEditDialog 初始化 thermocoupleType**

在 `openEditDialog` 函数中，初始化 `editForm` 时从第一个通道获取热电偶类型：

```typescript
  const ch0TcType = profile.channels.length > 0 && profile.channels[0].thermocoupleType
    ? profile.channels[0].thermocoupleType
    : 'K'
  editForm.value = {
    id: profile.id,
    name: profile.name,
    host: profile.host,
    port: profile.port,
    publishRate,
    unit: ch0Unit,
    precision: ch0Precision,
    autoConnect: (profile as any).autoConnect !== false,
    thermocoupleType: ch0TcType,
  }
```

- [ ] **Step 6: 深拷贝通道配置时包含 thermocoupleType**

`editChannels.value = profile.channels.map(c => ({ ...c }))` 已经会自动拷贝 `thermocoupleType`（因为使用了展开运算符），无需额外修改。

- [ ] **Step 7: 新增 syncThermocoupleTypeToChannels 函数**

在 `syncUnitToChannels` 函数之后添加：

```typescript
// 当统一热电偶类型变化时，同步到通道表格
function syncThermocoupleTypeToChannels() {
  if (editProfileType.value !== 'YX-DAQ-T') return
  for (const ch of editChannels.value) {
    ch.thermocoupleType = editForm.value.thermocoupleType
  }
}
```

- [ ] **Step 8: 修改通道表格 — 增加热电偶类型列**

在通道表格（约 L189-L225）中，在"单位"列之后添加热电偶类型列（仅 YX-DAQ-T 显示）：

```html
          <el-table-column v-if="editProfileType === 'YX-DAQ-T'" label="热电偶" width="90" align="center">
            <template #default="{ row }">
              <el-select v-model="row.thermocoupleType" size="small" style="width: 70px" @change="onChannelThermocoupleChange(row)">
                <el-option v-for="opt in thermocoupleTypeOptions" :key="opt.value" :label="opt.value" :value="opt.value" />
              </el-select>
            </template>
          </el-table-column>
```

- [ ] **Step 9: 新增 onChannelThermocoupleChange 函数**

当单个通道的热电偶类型改变时，如果所有通道类型相同，更新统一选择器；否则不做特殊处理：

```typescript
// 单个通道热电偶类型变化
function onChannelThermocoupleChange(_row: EditChannel) {
  // 检查是否所有通道类型一致
  const types = new Set(editChannels.value.map(c => c.thermocoupleType))
  if (types.size === 1) {
    editForm.value.thermocoupleType = editChannels.value[0].thermocoupleType
  }
}
```

- [ ] **Step 10: 修改 saveEdit — 保存时发送热电偶类型命令到硬件**

在 `saveEdit` 函数中，在现有的 `setUnit` 调用之后（约 L539-L545），添加热电偶类型设置逻辑：

```typescript
      // DAQ-T 设备：发送热电偶类型命令到硬件
      if (isTempDevice && oldStatus?.status === 'Connected') {
        // 构建热电偶类型字符串（16字符）
        const tcTypes = editChannels.value.map(c => c.thermocoupleType || 'K').join('').padEnd(16, 'K').substring(0, 16)
        const tcErr = await deviceStore.setThermocoupleType(formSnapshot.id, tcTypes)
        if (tcErr) {
          ElMessage.error(`设置热电偶类型失败: ${tcErr}`)
          return
        }
      }
```

- [ ] **Step 11: 修改 saveEdit — 更新通道配置时包含 thermocoupleType**

在 `saveEdit` 中构建 `updatedChannels` 时，添加 `thermocoupleType` 字段：

```typescript
    const updatedChannels = channelsSnapshot.map(c => ({
      index: c.index,
      name: c.name,
      enabled: c.enabled,
      unit: isTempDevice ? '°C' : (c.index === pc ? 'Pa' : (c.index === pc + 1 ? '°C' : formSnapshot.unit)),
      precision: formSnapshot.precision,
      rangeMin: c.rangeMin,
      rangeMax: c.rangeMax,
      thermocoupleType: isTempDevice ? (c.thermocoupleType || 'K') : undefined,
    }))
```

- [ ] **Step 12: 修改 syncUnitToChannels — YX-DAQ-T 不再从 editForm.unit 同步**

当前 `syncUnitToChannels` 对 YX-DAQ-T 已经做了特殊处理（固定 °C），无需修改。

- [ ] **Step 13: 修改添加设备对话框 — YX-DAQ-T 默认单位为 °C，增加 thermocoupleType**

在 `addDevice` 函数中，创建通道时添加 `thermocoupleType`：

```typescript
      if (info.isTemperature) {
        channels.push({
          index: i,
          name: `CH${i+1}`,
          enabled: true,
          unit: '°C',
          precision: newDevice.value.precision,
          rangeMin: -100,
          rangeMax: 300,
          thermocoupleType: 'K',
        })
      }
```

- [ ] **Step 14: 验证前端构建通过**

Run: `cd yx-daq/frontend && npm run build`
Expected: 构建成功

- [ ] **Step 15: Commit**

```bash
git add yx-daq/frontend/src/views/DeviceView.vue
git commit -m "feat: add thermocouple type selector to DAQ-T device edit dialog"
```

---

### Task 7: 连接后自动读取热电偶类型并更新前端

**Files:**
- Modify: `frontend/src/stores/device.ts`
- Modify: `frontend/src/views/DeviceView.vue`

- [ ] **Step 1: 在 device store 中监听 profile 变化以更新热电偶类型**

当设备连接成功后，`onConfigSynced` 回调会更新 profile 中的 `thermocoupleType` 字段（通过 Task 3 的实现），前端通过 `fetchProfiles()` 即可获取最新数据。

在 `connectDevice` 方法中，连接成功后已经调用了 `fetchStatuses()`，但还需要确保 `fetchProfiles()` 也被调用以获取更新后的通道配置：

```typescript
  async function connectDevice(id: string): Promise<string | null> {
    connectingIds.value = new Set([...connectingIds.value, id])
    try {
      await ConnectDevice(id)
      // 等待配置同步完成（驱动内部有 300ms 延迟 + 命令交互时间）
      await new Promise(resolve => setTimeout(resolve, 1000))
      await fetchProfiles()
      await fetchStatuses()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('connectDevice failed:', msg)
      return msg
    } finally {
      const newSet = new Set(connectingIds.value)
      newSet.delete(id)
      connectingIds.value = newSet
    }
  }
```

注意：1.5 秒延迟是因为 `syncHardwareConfig` 在 `Connect` 后有 `DAQTConfigSyncDelayMs` 的延迟。这个延迟是必要的，因为配置同步是异步的。

- [ ] **Step 2: 验证完整流程**

手动测试：
1. 添加 YX-DAQ-T 设备
2. 连接设备
3. 确认连接后通道配置中自动填充了从设备读取的热电偶类型
4. 编辑设备，确认热电偶类型选择器正常工作
5. 修改热电偶类型并保存，确认命令发送到硬件

- [ ] **Step 3: Commit**

```bash
git add yx-daq/frontend/src/stores/device.ts
git commit -m "feat: auto-fetch profiles after device connect for thermocouple type sync"
```

---

### Task 8: 集成验证

**Files:**
- 无新文件

- [ ] **Step 1: Go 编译检查**

Run: `cd yx-daq && go build ./...`
Expected: 编译成功

- [ ] **Step 2: 前端构建检查**

Run: `cd yx-daq/frontend && npm run build`
Expected: 构建成功

- [ ] **Step 3: Go lint 检查**

Run: `cd yx-daq && golangci-lint run ./internal/...`
Expected: 无新增 lint 错误

- [ ] **Step 4: 前端 lint 检查**

Run: `cd yx-daq/frontend && npm run lint`
Expected: 无新增 lint 错误

- [ ] **Step 5: 完整构建**

Run: `cd yx-daq && wails3 task build`
Expected: 生成 `build/bin/yx-daq.exe`

- [ ] **Step 6: Commit（如有修复）**

```bash
git add -A
git commit -m "fix: address integration issues from thermocouple type feature"
```
