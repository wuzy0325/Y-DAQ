package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/calibration"
	"yx-daq/internal/logger"
	"yx-daq/internal/manager"
	"yx-daq/internal/scanner"
	"yx-daq/internal/storage"
	"yx-daq/internal/three_hole"
	"yx-daq/internal/types"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 主应用结构
type App struct {
	ctx              context.Context
	deviceManager    *manager.DeviceManager
	motionManager    *manager.MotionControllerManager
	acquisitionHub   *manager.AcquisitionHub
	calibService     *calibration.CalibrationService
	threeHoleService *three_hole.ThreeHoleTraversalService
	configManager    *storage.ConfigManager
	dataStorage      *storage.DataStorageService
	pdfReport        *storage.PdfReportService
	daqScanner       *scanner.DAQScanner
	publishCancel    chan struct{}
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{
		deviceManager:  manager.NewDeviceManager(),
		motionManager:  manager.NewMotionControllerManager(),
		acquisitionHub: manager.NewAcquisitionHub(),
		pdfReport:      storage.NewPdfReportService(),
		daqScanner:     scanner.NewDAQScanner(),
		publishCancel:  make(chan struct{}),
	}
}

// Startup 应用启动
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	if err := logger.Init(); err != nil {
		slog.Error("logger init failed", "err", err)
	}

	// 初始化配置
	configDir := a.getConfigDir()
	a.configManager = storage.NewConfigManager(configDir)
	if err := a.configManager.LoadAll(); err != nil {
		slog.Error("load config failed", "err", err)
	}

	// 设置数据管道: 设备数据 → AcquisitionHub + DataStorage
	a.dataStorage = storage.NewDataStorageService(a.GetDataDir())
	a.deviceManager.SetDataSink(func(payload types.DataPayload) {
		a.acquisitionHub.OnData(payload)
		if a.dataStorage.IsRecording() {
			a.dataStorage.HandlePayload(payload)
		}
	})

	// 设置设备配置持久化
	a.deviceManager.SetConfigStore(a.configManager.Devices)

	// 初始化设备管理器
	a.deviceManager.Init()

	// 初始化运动控制器管理器（从配置文件加载）
	a.initMotionFromConfig()

	// 初始化校准服务
	a.calibService = calibration.NewCalibrationService(&CalibrationEventPublisher{ctx: a.ctx})
	a.calibService.SetDataGetter(func(deviceID string, channelIndex int) (float64, bool) {
		return a.deviceManager.GetChannelValue(deviceID, channelIndex)
	})
	a.calibService.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			if !ch.Enabled {
				continue
			}
			val, ok := a.acquisitionHub.GetLatestValue(a.calibService.GetStatus().TaskID, ch.Channel)
			if ok {
				result[ch.Channel] = val
			}
		}
		return result, nil
	})
	a.calibService.SetMotionController(func(axis types.AxisName, position float64) error {
		profiles := a.motionManager.GetProfiles()
		for _, p := range profiles {
			if a.motionManager.IsConnected(p.ID) {
				return a.motionManager.MoveTo(p.ID, axis, position)
			}
		}
		return fmt.Errorf("no motion controller connected")
	})

	// 初始化三孔移位测试服务
	a.threeHoleService = three_hole.NewThreeHoleTraversalService(&ThreeHoleEventPublisher{ctx: a.ctx})
	a.threeHoleService.SetBatchGetter(func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		config := a.threeHoleService.GetConfig()
		deviceID := config.DeviceID
		for _, ch := range channels {
			if !ch.Enabled {
				continue
			}
			if deviceID != "" {
				snapshots := a.acquisitionHub.GetSnapshot()
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
				snapshots := a.acquisitionHub.GetSnapshot()
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
	a.threeHoleService.SetMotionController(func(axis types.AxisName, position float64) error {
		config := a.threeHoleService.GetConfig()
		mcID := config.MotionControllerID
		if mcID != "" && a.motionManager.IsConnected(mcID) {
			return a.motionManager.MoveTo(mcID, axis, position)
		}
		if mcID == "" {
			slog.Warn("三孔测试: 未指定运动控制器，将使用第一个已连接的控制器")
		}
		profiles := a.motionManager.GetProfiles()
		for _, p := range profiles {
			if a.motionManager.IsConnected(p.ID) {
				return a.motionManager.MoveTo(p.ID, axis, position)
			}
		}
		return fmt.Errorf("no motion controller connected")
	})
	a.threeHoleService.SetMotionWaiter(func(axis types.AxisName, timeoutMs int) error {
		config := a.threeHoleService.GetConfig()
		mcID := config.MotionControllerID
		if mcID != "" && a.motionManager.IsConnected(mcID) {
			return a.motionManager.WaitForMotionComplete(mcID, axis, timeoutMs)
		}
		profiles := a.motionManager.GetProfiles()
		for _, p := range profiles {
			if a.motionManager.IsConnected(p.ID) {
				return a.motionManager.WaitForMotionComplete(p.ID, axis, timeoutMs)
			}
		}
		return fmt.Errorf("no motion controller connected")
	})

	// 启动数据快照发布
	a.acquisitionHub.SetOnSnapshot(func(snapshots []types.DataPayload) {
		wailsRuntime.EventsEmit(a.ctx, "daq:data-snapshot", snapshots) // TODO: 抽取为 EventPublisher 接口以解耦 Wails Runtime 依赖
	})
	go a.acquisitionHub.StartPublishing(a.publishCancel)

	// 启动运动状态轮询广播
	go a.broadcastMotionStatus()

	slog.Info("YX-DAQ application started")
}

// Shutdown 应用关闭
func (a *App) Shutdown(ctx context.Context) {
	a.publishCancel <- struct{}{}
	a.threeHoleService.Stop()
	a.threeHoleService.StopRealtimeMonitor()
	// 等待三孔测试 goroutine 退出，确保 CSV 数据完整写入（最多等3秒）
	shutdownDeadline := time.Now().Add(3 * time.Second)
	for a.threeHoleService.GetStatus().Status == types.TraversalStatusRunning && time.Now().Before(shutdownDeadline) {
		time.Sleep(50 * time.Millisecond)
	}
	a.dataStorage.StopRecording()
	a.deviceManager.StopAcquisitionAll()
	if a.configManager != nil {
		profiles := a.deviceManager.GetProfiles()
		if err := a.configManager.Devices.Set(profiles); err != nil {
			slog.Error("save device profiles on shutdown failed", "err", err)
		}
		motionProfiles := a.motionManager.GetProfiles()
		if err := a.configManager.Motion.Set(motionProfiles); err != nil {
			slog.Error("save motion profiles on shutdown failed", "err", err)
		}
	}
	slog.Info("YX-DAQ application shutdown")
	logger.Close()
}

// initMotionFromConfig 从配置文件初始化运动控制器
func (a *App) initMotionFromConfig() {
	if a.configManager != nil {
		savedProfiles := a.configManager.Motion.Get()
		if len(savedProfiles) > 0 {
			for _, p := range savedProfiles {
				a.motionManager.AddProfile(p)
			}
			slog.Info("loaded motion controller profiles from config", "count", len(savedProfiles))

			profiles := a.motionManager.GetProfiles()
			hasB140 := false
			for _, p := range profiles {
				if p.Type == types.MotionTypeB140 {
					hasB140 = true
					break
				}
			}
			if !hasB140 {
				defaultAxes := []types.AxisConfig{
					{Name: types.AxisX, Enabled: true, Kind: types.AxisKindLinear, Inverted: false, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5, MaxSpeed: 50, EncoderScale: 0.005},
					{Name: types.AxisY, Enabled: true, Kind: types.AxisKindLinear, Inverted: false, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5, MaxSpeed: 50, EncoderScale: 0.005},
					{Name: types.AxisZ, Enabled: true, Kind: types.AxisKindLinear, Inverted: false, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5, MaxSpeed: 50, EncoderScale: 0.005},
					{Name: types.AxisU, Enabled: true, Kind: types.AxisKindRotary, Inverted: false, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 4, MaxSpeed: 30, EncoderScale: 0.005},
				}
				b140Profile := types.MotionControllerProfile{
					ID:        "b140-mc-1",
					Name:      "B140 运动控制器",
					Type:      types.MotionTypeB140,
					Address:   "192.168.1.101",
					Port:      5000,
					TimeoutMs: 5000,
					Axes:      defaultAxes,
				}
				a.motionManager.AddProfile(b140Profile)
				slog.Info("added default B140 motion controller profile")
			}

			go a.motionManager.StartPolling()
			for _, p := range a.motionManager.GetProfiles() {
				if p.Type == types.MotionTypeSimulated {
					if err := a.motionManager.Connect(p.ID); err != nil {
						slog.Error("auto-connect simulated motion controller failed", "err", err)
					}
				}
			}
			return
		}
	}

	a.motionManager.Init()
}

// saveMotionConfig 保存运动控制器配置到文件
func (a *App) saveMotionConfig() {
	if a.configManager == nil {
		return
	}
	profiles := a.motionManager.GetProfiles()
	if err := a.configManager.Motion.Set(profiles); err != nil {
		slog.Error("save motion config failed", "err", err)
	}
}

// broadcastMotionStatus 广播运动状态
func (a *App) broadcastMotionStatus() {
	ticker := time.NewTicker(time.Duration(types.MotionPollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-a.publishCancel:
			return
		case <-ticker.C:
			statuses := a.motionManager.GetStatusAll()
			wailsRuntime.EventsEmit(a.ctx, "motion:status-updated", statuses) // TODO: 抽取为 EventPublisher 接口以解耦 Wails Runtime 依赖
		}
	}
}

// getConfigDir 获取配置目录路径
func (a *App) getConfigDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.TempDir(), "yx-daq-config")
	}
	return filepath.Join(configDir, "yx-daq")
}

// GetDataDir 获取数据存储目录路径
func (a *App) GetDataDir() string {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		dataDir = filepath.Join(os.TempDir(), "yx-daq-data")
	}
	return filepath.Join(dataDir, "yx-daq", "data")
}

