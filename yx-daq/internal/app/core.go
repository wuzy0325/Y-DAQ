package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"yx-daq/internal/calibration"
	"yx-daq/internal/five_hole"
	"yx-daq/internal/logger"
	"yx-daq/internal/manager"
	"yx-daq/internal/scanner"
	"yx-daq/internal/storage"
	"yx-daq/internal/three_hole"
	"yx-daq/internal/types"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Core 核心业务管理器，持有所有子系统和 *application.Application 引用
type Core struct {
	App               *application.App
	DeviceManager     *manager.DeviceManager
	MotionManager     *manager.MotionControllerManager
	AcquisitionHub    *manager.AcquisitionHub
	CalibService      *calibration.CalibrationService
	ThreeHoleServices map[string]*three_hole.ThreeHoleTraversalService // probe1, probe2
	FiveHoleService   *five_hole.FiveHoleTraversalService             // 五孔单实例管理 1-3 探针
	ConfigManager     *storage.ConfigManager
	DataStorage       *storage.DataStorageService
	PdfReport         *storage.PdfReportService
	DaqScanner        *scanner.DAQScanner
	publishCancel     chan struct{}

	threeHoleMotionMu sync.Mutex
	fiveHoleMotionMu  sync.Mutex
}

// NewCore 创建核心业务管理器
func NewCore() *Core {
	return &Core{
		DeviceManager:     manager.NewDeviceManager(),
		MotionManager:     manager.NewMotionControllerManager(),
		AcquisitionHub:    manager.NewAcquisitionHub(),
		PdfReport:         storage.NewPdfReportService(),
		DaqScanner:        scanner.NewDAQScanner(),
		ThreeHoleServices: make(map[string]*three_hole.ThreeHoleTraversalService),
		publishCancel:     make(chan struct{}),
	}
}

// Startup 初始化所有子系统
func (c *Core) Startup(app *application.App) {
	c.App = app

	if err := logger.Init(); err != nil {
		slog.Error("logger init failed", "err", err)
	}

	configDir := c.getConfigDir()
	c.ConfigManager = storage.NewConfigManager(configDir)
	if err := c.ConfigManager.LoadAll(); err != nil {
		slog.Error("load config failed", "err", err)
	}

	c.DataStorage = storage.NewDataStorageService(c.GetDataDir())
	c.DeviceManager.SetDataSink(func(payload types.DataPayload) {
		c.AcquisitionHub.OnData(payload)
		if c.DataStorage.IsRecording() {
			c.DataStorage.HandlePayload(payload)
		}
	})

	c.DeviceManager.SetConfigStore(c.ConfigManager.Devices)
	c.DeviceManager.SetOnStatusChange(func(statuses []types.DeviceStatus) {
		c.App.Event.Emit("device:status-updated", statuses)
	})
	c.DeviceManager.Init()

	c.MotionManager.SetConfigStore(c.ConfigManager.Motion)
	c.MotionManager.SetOnStatusChange(func(statuses []types.MotionControllerStatus) {
		c.App.Event.Emit("motion:status-updated", statuses)
	})
	c.MotionManager.Init()
	c.initCalibration()
	c.initThreeHole()
	c.initFiveHole()

	// 仅在快照非空、或由非空转为空的边界处推送，避免无设备采集时以 20Hz
	// 反复推送空数组造成前端无谓的响应式刷新与 IPC 开销。
	var lastSnapshotEmpty bool
	c.AcquisitionHub.SetOnSnapshot(func(snapshots []types.DataPayload) {
		if len(snapshots) == 0 {
			if lastSnapshotEmpty {
				return
			}
			lastSnapshotEmpty = true
		} else {
			lastSnapshotEmpty = false
		}
		c.App.Event.Emit("daq:data-snapshot", snapshots)
	})
	go c.AcquisitionHub.StartPublishing(c.publishCancel)

	go c.broadcastMotionStatus()
	go c.broadcastDeviceStatus()
	slog.Info("YX-DAQ application started")
}

// Shutdown 关闭所有子系统，保存配置
func (c *Core) Shutdown() {
	// 关闭 publishCancel 通道，通知所有监听协程退出
	// （StartPublishing、broadcastMotionStatus、broadcastDeviceStatus 都监听此通道）
	if c.publishCancel != nil {
		close(c.publishCancel)
		c.publishCancel = nil
	}
	for _, svc := range c.ThreeHoleServices {
		svc.Stop()
		svc.StopRealtimeMonitor()
	}
	if c.FiveHoleService != nil {
		c.FiveHoleService.Stop()
		c.FiveHoleService.StopRealtimeMonitor()
	}
	if c.DataStorage != nil {
		c.DataStorage.StopRecording()
	}
	if c.DeviceManager != nil {
		c.DeviceManager.StopAcquisitionAll()
		// 断开所有设备连接，关闭 TCP 连接并停止接收协程
		for _, p := range c.DeviceManager.GetProfiles() {
			c.DeviceManager.Disconnect(p.ID)
		}
	}
	// 停止运动控制器轮询并断开所有连接
	c.MotionManager.StopPolling()
	for _, p := range c.MotionManager.GetProfiles() {
		c.MotionManager.Disconnect(p.ID)
	}
	if c.ConfigManager != nil {
		if c.DeviceManager != nil {
			profiles := c.DeviceManager.GetProfiles()
			if err := c.ConfigManager.Devices.Set(profiles); err != nil {
				slog.Error("save device profiles on shutdown failed", "err", err)
			}
		}
		motionProfiles := c.MotionManager.GetProfiles()
		if err := c.ConfigManager.Motion.Set(motionProfiles); err != nil {
			slog.Error("save motion profiles on shutdown failed", "err", err)
		}
	}
	slog.Info("YX-DAQ application shutdown")
	logger.Close()
	// 强制退出：Wails v3 在 Windows 下的关闭流程可能因 InvokeSync 嵌套或消息循环
	// 未退出导致 a.Run() 不返回，main() 中的 os.Exit(0) 永远无法执行。
	// 在所有资源清理完毕后直接退出进程，确保不残留。
	os.Exit(0)
}

func (c *Core) initCalibration() {
	c.CalibService = calibration.NewCalibrationService(&CalibrationEventPublisher{app: c.App})
	c.CalibService.SetDataGetter(func(deviceID string, channelIndex int) (float64, bool) {
		return c.DeviceManager.GetChannelValue(deviceID, channelIndex)
	})
	c.CalibService.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			if !ch.Enabled {
				continue
			}
			val, ok := c.AcquisitionHub.GetLatestValue(c.CalibService.GetStatus().TaskID, ch.Channel)
			if ok {
				result[ch.Channel] = val
			}
		}
		return result, nil
	})
	c.CalibService.SetMotionController(func(axis types.AxisName, position float64) error {
		profiles := c.MotionManager.GetProfiles()
		for _, p := range profiles {
			if c.MotionManager.IsConnected(p.ID) {
				return c.MotionManager.MoveTo(p.ID, axis, position)
			}
		}
		return fmt.Errorf("no motion controller connected")
	})
}

func (c *Core) initThreeHole() {
	for _, probeID := range []string{"probe1", "probe2"} {
		svc := c.createThreeHoleService(probeID)
		c.ThreeHoleServices[probeID] = svc
	}
}

// initFiveHole 初始化五孔移位测试服务（单实例，管理 1-3 探针）
func (c *Core) initFiveHole() {
	svc := five_hole.NewFiveHoleTraversalService(&FiveHoleEventPublisher{app: c.App})

	// 多设备批量读取：按 DeviceID 从 AcquisitionHub 获取快照，按通道索引取值
	// 返回 (通道值map, 设备最新数据时间戳, error)，timestamp 用于采样时判断数据新鲜度
	svc.SetMultiDeviceBatchGetter(func(deviceID string, channels []int) (map[int]float64, int64, error) {
		result := make(map[int]float64)
		if deviceID == "" {
			return result, 0, nil
		}
		snapshots := c.AcquisitionHub.GetSnapshot()
		for _, snap := range snapshots {
			if snap.DeviceID != deviceID {
				continue
			}
			for _, wantCh := range channels {
				for i, idx := range snap.ChannelIndices {
					if idx == wantCh && i < len(snap.Channels) {
						result[wantCh] = snap.Channels[i]
						break
					}
				}
			}
			return result, snap.Timestamp, nil
		}
		return result, 0, nil
	})

	// 每轴独立运动控制（每轴独立 ControllerID）
	svc.SetProbeAxisMover(func(controllerID string, axis types.AxisName, position float64) error {
		c.fiveHoleMotionMu.Lock()
		defer c.fiveHoleMotionMu.Unlock()
		if controllerID != "" && c.MotionManager.IsConnected(controllerID) {
			return c.MotionManager.MoveTo(controllerID, axis, position)
		}
		return fmt.Errorf("位移机构 %s 未连接", controllerID)
	})

	svc.SetProbeAxisWaiter(func(controllerID string, axis types.AxisName, timeoutMs int) error {
		c.fiveHoleMotionMu.Lock()
		defer c.fiveHoleMotionMu.Unlock()
		if controllerID != "" && c.MotionManager.IsConnected(controllerID) {
			return c.MotionManager.WaitForMotionComplete(controllerID, axis, timeoutMs)
		}
		return fmt.Errorf("位移机构 %s 未连接", controllerID)
	})

	c.FiveHoleService = svc
}

// FiveHoleEventPublisher 五孔事件发布器
type FiveHoleEventPublisher struct {
	app *application.App
}

func (p *FiveHoleEventPublisher) EmitProgress(event types.FiveHoleTraversalProgressEvent) {
	p.app.Event.Emit("five-hole:progress", event)
}

func (p *FiveHoleEventPublisher) EmitRealtime(event types.FiveHoleTraversalRealtimeEvent) {
	p.app.Event.Emit("five-hole:realtime", event)
}

func (p *FiveHoleEventPublisher) EmitComplete(event types.FiveHoleTraversalCompleteEvent) {
	p.app.Event.Emit("five-hole:complete", event)
}

func (p *FiveHoleEventPublisher) EmitError(event types.FiveHoleTraversalErrorEvent) {
	p.app.Event.Emit("five-hole:error", event)
}

func (c *Core) createThreeHoleService(probeID string) *three_hole.ThreeHoleTraversalService {
	svc := three_hole.NewThreeHoleTraversalService(&ThreeHoleEventPublisher{app: c.App, probeID: probeID})
	svc.SetBatchGetter(func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		config := svc.GetConfig()
		deviceID := config.DeviceID
		for _, ch := range channels {
			if !ch.Enabled {
				continue
			}
			if deviceID != "" {
				snapshots := c.AcquisitionHub.GetSnapshot()
				for _, snap := range snapshots {
					if snap.DeviceID != deviceID {
						continue
					}
					for i, idx := range snap.ChannelIndices {
						if idx == ch.Channel && i < len(snap.Channels) {
							result[ch.Channel] = snap.Channels[i]
							break
						}
					}
				}
			} else {
				snapshots := c.AcquisitionHub.GetSnapshot()
				for _, snap := range snapshots {
					for i, idx := range snap.ChannelIndices {
						if idx == ch.Channel && i < len(snap.Channels) {
							result[ch.Channel] = snap.Channels[i]
							break
						}
					}
					if _, ok := result[ch.Channel]; ok {
						break
					}
				}
			}
		}
		return result, nil
	})
	svc.SetMotionController(func(axis types.AxisName, position float64) error {
		c.threeHoleMotionMu.Lock()
		defer c.threeHoleMotionMu.Unlock()
		config := svc.GetConfig()
		mcID := config.MotionControllerID
		if mcID != "" && c.MotionManager.IsConnected(mcID) {
			return c.MotionManager.MoveTo(mcID, axis, position)
		}
		if mcID == "" {
			slog.Warn("三孔测试: 未指定运动控制器，将使用第一个已连接的控制器")
		}
		profiles := c.MotionManager.GetProfiles()
		for _, p := range profiles {
			if c.MotionManager.IsConnected(p.ID) {
				return c.MotionManager.MoveTo(p.ID, axis, position)
			}
		}
		return fmt.Errorf("no motion controller connected")
	})
	svc.SetMotionWaiter(func(axis types.AxisName, timeoutMs int) error {
		c.threeHoleMotionMu.Lock()
		defer c.threeHoleMotionMu.Unlock()
		config := svc.GetConfig()
		mcID := config.MotionControllerID
		if mcID != "" && c.MotionManager.IsConnected(mcID) {
			return c.MotionManager.WaitForMotionComplete(mcID, axis, timeoutMs)
		}
		profiles := c.MotionManager.GetProfiles()
		for _, p := range profiles {
			if c.MotionManager.IsConnected(p.ID) {
				return c.MotionManager.WaitForMotionComplete(p.ID, axis, timeoutMs)
			}
		}
		return fmt.Errorf("no motion controller connected")
	})
	return svc
}

func (c *Core) broadcastMotionStatus() {
	ticker := time.NewTicker(time.Duration(types.MotionPollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	var lastJSON string
	for {
		select {
		case <-c.publishCancel:
			return
		case <-ticker.C:
			statuses := c.MotionManager.GetCachedStatusAll()
			// 仅在状态变化时推送：运动控制器静止时轮询值不变，避免 10Hz
			// 重复推送相同状态造成前端 axisUIStates 的无谓响应式刷新。
			curJSON, _ := json.Marshal(statuses)
			if string(curJSON) == lastJSON {
				continue
			}
			lastJSON = string(curJSON)
			c.App.Event.Emit("motion:status-updated", statuses)
		}
	}
}

// broadcastDeviceStatus 定时推送设备状态，确保前端注册监听后能立即收到初始状态。
// 采用变化检测：仅在状态发生变化时推送，避免每 500ms 重复推送相同状态造成前端
// 无谓刷新。初始状态首次必然变化（lastJSON 为空），故仍能完成初始下发；前端另有
// fetchStatuses() + 800ms 重试作为兜底，状态变化时由 emitStatusChange 实时推送。
func (c *Core) broadcastDeviceStatus() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastJSON string
	for {
		select {
		case <-c.publishCancel:
			return
		case <-ticker.C:
			statuses := c.DeviceManager.GetStatusAll()
			curJSON, _ := json.Marshal(statuses)
			if string(curJSON) == lastJSON {
				continue
			}
			lastJSON = string(curJSON)
			c.App.Event.Emit("device:status-updated", statuses)
		}
	}
}

func (c *Core) getConfigDir() string {
	// 优先使用可执行文件所在目录下的 config 子目录（兼容沙箱环境）
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "config")
		if logger.TryEnsureDir(candidate) {
			return candidate
		}
	}

	// 回退到系统标准目录
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.TempDir(), "yx-daq-config")
	}
	return filepath.Join(configDir, "yx-daq")
}

// GetDataDir 获取数据存储目录路径
func (c *Core) GetDataDir() string {
	if c.ConfigManager != nil {
		cfg := c.ConfigManager.Storage.Get()
		if cfg.DataSavePath != "" {
			return cfg.DataSavePath
		}
	}

	// 优先使用可执行文件所在目录下的 data 子目录（兼容沙箱环境）
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "data")
		if logger.TryEnsureDir(candidate) {
			return candidate
		}
	}

	// 回退到系统标准目录
	dataDir, err := os.UserConfigDir()
	if err != nil {
		dataDir = filepath.Join(os.TempDir(), "yx-daq-data")
	}
	return filepath.Join(dataDir, "yx-daq", "data")
}

// GetThreeHoleService 获取指定探针的服务
func (c *Core) GetThreeHoleService(probeID string) *three_hole.ThreeHoleTraversalService {
	return c.ThreeHoleServices[probeID]
}

// CheckThreeHoleMotionConflict 检查指定探针的运动控制器是否被其他正在运行的探针占用
func (c *Core) CheckThreeHoleMotionConflict(probeID string, mcID string) error {
	if mcID == "" {
		return nil
	}
	for id, svc := range c.ThreeHoleServices {
		if id == probeID {
			continue
		}
		otherMcID := svc.GetConfig().MotionControllerID
		if otherMcID == mcID && svc.GetStatus().Status == types.TraversalStatusRunning {
			return fmt.Errorf("运动控制器 %s 正被探针 %s 使用中，不能同时启动", mcID, id)
		}
	}
	return nil
}

// CheckThreeHoleDeviceChannelOverlap 检查同一采集设备上不同探针的通道映射是否冲突
func (c *Core) CheckThreeHoleDeviceChannelOverlap(probeID string, deviceID string, channels []types.ThreeHoleProbeChannelConfig) string {
	if deviceID == "" {
		return ""
	}
	for id, svc := range c.ThreeHoleServices {
		if id == probeID {
			continue
		}
		otherCfg := svc.GetConfig()
		if otherCfg.DeviceID != deviceID {
			continue
		}
		if svc.GetStatus().Status != types.TraversalStatusRunning {
			continue
		}
		myChannels := make(map[int]string)
		for _, ch := range channels {
			if ch.Enabled {
				myChannels[ch.Channel] = string(ch.Role)
			}
		}
		for _, otherCh := range otherCfg.ProbeChannels {
			if !otherCh.Enabled {
				continue
			}
			if myRole, ok := myChannels[otherCh.Channel]; ok && myRole != string(otherCh.Role) {
				return fmt.Sprintf("警告: 采集设备 %s 的通道 %d 被探针 %s 映射为 %s，当前探针映射为 %s，数据可能冲突",
					deviceID, otherCh.Channel, id, string(otherCh.Role), myRole)
			}
		}
	}
	return ""
}

// EmergencyStopWithRetry 急停运动控制器（重试1次）
func (c *Core) EmergencyStopWithRetry(mcID string) {
	if err := c.MotionManager.EmergencyStop(mcID); err != nil {
		slog.Warn("急停失败，重试1次", "mcID", mcID, "err", err)
		time.Sleep(100 * time.Millisecond)
		if err2 := c.MotionManager.EmergencyStop(mcID); err2 != nil {
			slog.Error("急停重试仍失败", "mcID", mcID, "err", err2)
		}
	}
}
