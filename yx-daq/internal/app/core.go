package app

import (
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
	ConfigManager     *storage.ConfigManager
	DataStorage       *storage.DataStorageService
	PdfReport         *storage.PdfReportService
	DaqScanner        *scanner.DAQScanner
	publishCancel     chan struct{}
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
	c.DeviceManager.Init()

	c.initMotionFromConfig()
	c.initCalibration()
	c.initThreeHole()

	c.AcquisitionHub.SetOnSnapshot(func(snapshots []types.DataPayload) {
		c.App.Event.Emit("daq:data-snapshot", snapshots)
	})
	go c.AcquisitionHub.StartPublishing(c.publishCancel)

	go c.broadcastMotionStatus()
	slog.Info("YX-DAQ application started")
}

// Shutdown 关闭所有子系统，保存配置
func (c *Core) Shutdown() {
	if c.publishCancel != nil {
		c.publishCancel <- struct{}{}
	}
	for _, svc := range c.ThreeHoleServices {
		svc.Stop()
		svc.StopRealtimeMonitor()
	}
	if c.DataStorage != nil {
		c.DataStorage.StopRecording()
	}
	if c.DeviceManager != nil {
		c.DeviceManager.StopAcquisitionAll()
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

func (c *Core) initMotionFromConfig() {
	if c.ConfigManager != nil {
		savedProfiles := c.ConfigManager.Motion.Get()
		if len(savedProfiles) > 0 {
			for _, p := range savedProfiles {
				c.MotionManager.AddProfile(p)
			}
			slog.Info("loaded motion controller profiles from config", "count", len(savedProfiles))

			profiles := c.MotionManager.GetProfiles()
			hasB140 := false
			for _, p := range profiles {
				if p.Type == types.MotionTypeB140 {
					hasB140 = true
					break
				}
			}
			if !hasB140 {
				b140Profile := types.MotionControllerProfile{
					ID:        "b140-mc-1",
					Name:      "B140 运动控制器",
					Type:      types.MotionTypeB140,
					Address:   "192.168.1.101",
					Port:      5000,
					TimeoutMs: 5000,
					Axes:      types.DefaultAxisConfigs(),
				}
				c.MotionManager.AddProfile(b140Profile)
			}

			go c.MotionManager.StartPolling()
			for _, p := range c.MotionManager.GetProfiles() {
				switch p.Type {
				case types.MotionTypeSimulated:
					if err := c.MotionManager.Connect(p.ID); err != nil {
						slog.Error("auto-connect simulated motion controller failed", "err", err)
					}
				case types.MotionTypeB140:
					if err := c.MotionManager.Connect(p.ID); err != nil {
						slog.Warn("auto-connect B140 failed", "id", p.ID, "address", p.Address, "err", err)
					}
				}
			}
			return
		}
	}
	c.MotionManager.Init()
}

func (c *Core) broadcastMotionStatus() {
	ticker := time.NewTicker(time.Duration(types.MotionPollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.publishCancel:
			return
		case <-ticker.C:
			statuses := c.MotionManager.GetCachedStatusAll()
			c.App.Event.Emit("motion:status-updated", statuses)
		}
	}
}

func (c *Core) saveMotionConfig() {
	if c.ConfigManager == nil {
		return
	}
	profiles := c.MotionManager.GetProfiles()
	if err := c.ConfigManager.Motion.Set(profiles); err != nil {
		slog.Error("save motion config failed", "err", err)
	}
}

func (c *Core) getConfigDir() string {
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
