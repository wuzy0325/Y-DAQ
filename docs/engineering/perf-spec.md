# YX-DAQ 高性能采集架构设计

## 一、性能目标

| 指标 | 目标 |
|------|------|
| 单设备采样率 | 1000 Hz |
| 最大设备数 | 10 台 |
| 总帧率 | 10,000 帧/秒 |
| 单帧数据量 | ~200 B（18 channel × 4 B float + 帧头） |
| 总吞吐量 | ~2 MB/s |
| UI 刷新率 | 1–10 Hz 可调 |
| 存储带宽 | 持续 2 MB/s 写入 |
| 运行时长 | 7×24 小时无重启 |

---

## 二、数据管道架构

```
设备 1 TCP ─→ recvLoop ─┐
设备 2 TCP ─→ recvLoop ─┤
...                       ├── [chan Frame] ──→ FanOut ──→ [chan Frame] ──→ StorageWriter (批量写入)
设备 10 TCP ─→ recvLoop ─┘       ↓
                              Downsampler ──→ [chan Snapshot] ──→ UI (1-10 Hz)
```

### 2.1 接收层（每设备独立 goroutine）

```
接收 goroutine (per device):
  recvLoop()
  ├── TCP read (conn.Read / bufio)
  ├── 拆包（2 字节大端长度前缀）
  ├── 解析 float32 × N channels
  └── types.DataPayload ─→ 发往公共 channel
```

- 每设备独立 `goroutine` + 独立 `net.Conn`
- 拆包完成立即发送，不阻塞
- channel `cap = 2048` 应对瞬时峰值（200 ms 缓冲量）

### 2.2 扇出层（FanOut）

接收一个 `chan *types.DataPayload`，扇出到 N 个消费者：

```
FanOut(input <-chan *types.DataPayload)
  ├── outputStorage chan<- *types.DataPayload  (cap=4096)
  ├── outputUI      chan<- *types.DataPayload  (cap=1024)
  └── 非阻塞尝试发送，防止慢消费者阻塞快消费者
```

扇出策略：

```go
for frame := range input {
    select {
    case outputStorage <- frame:
    default:
        // 存储队列满 → 丢帧（记录计数器，不阻塞）
    }
    select {
    case outputUI <- frame:
    default:
        // UI 队列满 → 丢帧（反正要降采样）
    }
}
```

### 2.3 UI 降采样层

```
Downsampler(input <-chan *types.DataPayload)
  输出: chan []types.DataPayload (1-10 Hz)
  算法: 周期内保留最后一帧，其余丢弃
```

- 频率可配置（1–10 Hz）
- 接收端只保留最新全量快照
- 不做滑动平均（UI 端如需平滑自行处理）

### 2.4 存储层

```
StorageWriter(input <-chan *types.DataPayload)
  批量缓冲: []*types.DataPayload (固定 2000 帧 = 0.2s × 10 设备)
  定时 flush: 每 200ms 或 缓冲区满 2000 帧触发写入
  文件: 二进制格式，按小时分片
```

**格式选择：二进制（非 CSV）**

| 维度 | CSV | 二进制 |
|------|-----|--------|
| 每帧写入 | 18 行字符串 | 1 条记录 |
| 写入速度（估算） | ~500 KB/s | ~4 MB/s |
| 文件大小（1h@10dev） | ~4.5 GB | ~800 MB |
| 随机读取 | 逐行解析 | 按偏移跳转 |
| 人眼可读 | 可读 | 不可读 |

**推荐二进制格式：**

```
[Header 64B]
  Magic:     uint32  (0x59445844 = "YAXD")
  Version:   uint8   (1)
  DeviceCount: uint8
  ChannelCount: uint16  (per-device max)
  FrameIntervalUs: uint32  (1000 = 1000Hz)
  Reserved:  [20]byte

[Frame Record 可变长]
  Timestamp:   int64  (unix ms)
  DeviceIDLen: uint8
  DeviceID:    [DeviceIDLen]byte
  ChannelData: [N × float32]  (大端序，与网络协议一致)
  CRC32:       uint32

每帧长度 ≈ 8 + 1 + DeviceIDLen + N×4 + 4
   ≈ 8 + 1 + 8 + 72 + 4 = 93 bytes (10 通道)
   ≈ 8 + 1 + 8 + 72 + 4 = 93 bytes (18 通道) = 同值
```

**文件分片策略：**

```
~/.yx-daq/recordings/
├── 2026-04-26/
│   ├── 10-00-00.yxd      # 10:00 - 10:59
│   ├── 11-00-00.yxd      # 11:00 - 11:59
│   └── ...
└── index.json             # 索引（文件名 ↔ 起止时间 ↔ 设备列表）
```

---

## 三、内存预算

| 组件 | 缓冲深度 | 单元素大小 | 总占用 |
|------|---------|-----------|--------|
| 接收 channel（×10 设备） | 2048 | ~200 B | ~4 MB |
| 扇出 storage channel | 4096 | ~200 B | ~0.8 MB |
| 扇出 UI channel | 1024 | ~200 B | ~0.2 MB |
| 存储写入缓冲 | 2000 | ~200 B | ~0.4 MB |
| UI 历史数据窗口（固定） | 10000 帧 | ~200 B | ~2 MB |
| **常驻上限** | | | **~10 MB** |

采集过程中无持续增长点，可长期运行。

---

## 四、稳定性设计

### 4.1 设备断线重连

沿用现有指数退避（`internal/driver/xy_daq16.go`）：

```
重连间隔: 1s → 2s → 4s → 8s → ... → 上限 60s
```

重连期间：
- 停止该设备接收 goroutine
- 存储和 UI 继续处理其他设备
- 重连成功后重新启动接收 goroutine

### 4.2 存储异常处理

```go
// flush 失败策略
func (w *Writer) flush() error {
    // 1. 重试 3 次
    // 2. 全部失败 → 尝试切新文件
    // 3. 新文件也失败 → 丢帧，记错误计数，继续
    // 4. 不会 panic，不会阻塞采集
}
```

### 4.3 启动恢复

- 记录 `index.json` 在启动时校验
- 上一次未关闭的文件在启动时自动修复尾部（补全 CRC）
- 采集中断的文件不丢失已有数据

### 4.4 优雅关闭

```go
// app.shutdown():
// 1. signal stop 给所有采集 goroutine（非阻塞）
// 2. 等待存储 writer flush（带 3s 超时）
// 3. 写 index.json 最终状态
// 4. 关闭所有设备连接
```

---

## 五、需要新增的组件

| 组件 | 说明 | 优先级 |
|------|------|--------|
| `internal/pipe/fanout.go` | 帧扇出器（1:N channel 分发） | P0 |
| `internal/pipe/downsampler.go` | UI 降采样（可配 1–10 Hz） | P0 |
| `internal/storage/binary_writer.go` | 二进制录制（.yxd 格式） | P0 |
| `internal/storage/index.go` | 录制索引管理 | P1 |
| `internal/storage/player.go` | 回放器（按时间范围读取 .yxd） | P1 |
| `internal/pipe/buffer.go` | 固定大小环形缓冲（UI 历史窗口） | P1 |

### 5.1 FanOut 接口

```go
package pipe

type FanOut struct {
    input  chan *types.DataPayload
    sinks  []chan<- *types.DataPayload
    dropped atomic.Int64  // 丢帧计数（调试用）
}

func NewFanOut(size int, sinks ...chan<- *types.DataPayload) *FanOut
func (f *FanOut) Start()
func (f *FanOut) Stop()
```

### 5.2 Downsampler 接口

```go
type Downsampler struct {
    input     chan *types.DataPayload
    output    chan []types.DataPayload
    interval  time.Duration  // 100ms for 10Hz
}

func NewDownsampler(input chan *types.DataPayload, hz int) *Downsampler
func (d *Downsampler) Output() <-chan []types.DataPayload
func (d *Downsampler) SetHz(hz int)
func (d *Downsampler) Start(cancel <-chan struct{})
```

### 5.3 BinaryWriter 接口

```go
type BinaryWriter struct {
    dir     string
    input   <-chan *types.DataPayload
    buffer  []*types.DataPayload
}

func NewBinaryWriter(dir string, input <-chan *types.DataPayload) *BinaryWriter
func (w *BinaryWriter) StartRecording() error
func (w *BinaryWriter) StopRecording() error
func (w *BinaryWriter) IsRecording() bool
```

---

## 六、现有代码修改清单

### 6.1 `app.go`

当前 `deviceManager.SetDataSink(...)` 是同步回调。改为：

```go
// 旧：同步回调（阻塞）
a.deviceManager.SetDataSink(func(payload types.DataPayload) { ... })

// 新：每设备独立 channel
pipeCh := make(chan *types.DataPayload, 2048)
fanOut := pipe.NewFanOut(pipeCh, storageCh, uiCh)
deviceManager.SetDataOutput(pipeCh)  // 驱动直接发送到 channel
```

### 6.2 `AcquisitionHub`

- 不再直接从 `dataSink` 接收
- 改为从 `downsampler.Output()` 读取

### 6.3 `DataStorageService`

- 保留 CSV export 功能（用于校准结果导出）
- 录制改用 `BinaryWriter`（持续高吞吐）
- CSV 只在单点测试 / 小数据量时使用

### 6.4 前端

- 当前 `daq:data-snapshot` 事件推送频率改为可调（1–10 Hz）
- 前端不需要改（已经通过事件驱动）

---

## 七、性能验证清单

```bash
# 1. 编译无错误
go build ./...

# 2. 模拟 10 设备 1000Hz 压力测试
cd frontend && npm run build
go test -bench=BenchmarkFanOut ./internal/pipe/
go test -bench=BenchmarkBinaryWriter ./internal/storage/

# 3. 内存监控
go test -bench=BenchmarkLongRun -timeout=30m ./internal/pipe/

# 4. 全链路模拟测试
go test -run TestAcquisitionPipeline ./internal/...
```

---

## 八、现阶段不需要做的

| 事项 | 理由 |
|------|------|
| protobuf/flatbuffers 序列化 | 当前 2 MB/s 吞吐不需要，增加复杂度 |
| gRPC 流式传输 | 当前纯单机桌面应用 |
| 数据库存储（SQLite/InfluxDB） | 二进制文件更简单，数据导出后离线处理 |
| WebSocket 推送到 Web | 无 Web 端需求 |
| 压缩存储（gzip/zstd） | 写入时压缩拖慢速度，可在导出时按需压缩 |
