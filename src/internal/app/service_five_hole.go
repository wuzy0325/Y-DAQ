package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/five_hole"
	"yx-daq/internal/types"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// FiveHoleService 五孔移位插值测试服务（Wails 绑定层，单实例管理 1-3 探针）
type FiveHoleService struct {
	Core *Core
}

// OpenTestWindow 打开五孔测试窗口（单窗口，非三孔多窗口模式）
func (s *FiveHoleService) OpenTestWindow() string {
	winName := "five-hole-test"
	title := "五孔移位插值测试"

	if existing, ok := s.Core.App.Window.GetByName(winName); ok {
		existing.Focus()
		return "focused"
	}

	win := s.Core.App.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             winName,
		Title:            title,
		Width:            1600, Height: 960,
		MinWidth:         1280, MinHeight: 800,
		BackgroundColour: application.NewRGB(10, 10, 26),
		URL:              "/#/five-hole-test",
	})
	win.Show()
	return "opened"
}

// LoadFiveHoleCalibFiles 加载五孔校准文件（按探针独立载入）
func (s *FiveHoleService) LoadFiveHoleCalibFiles(probeID string, filePaths []string) error {
	return s.Core.FiveHoleService.LoadCalibFiles(probeID, filePaths)
}

// IsFiveHoleCalibLoaded 五孔校准文件是否已加载
func (s *FiveHoleService) IsFiveHoleCalibLoaded(probeID string) bool {
	return s.Core.FiveHoleService.IsCalibLoaded(probeID)
}

// GetFiveHoleCalibInfo 获取五孔校准文件信息
func (s *FiveHoleService) GetFiveHoleCalibInfo(probeID string) []types.FiveHoleCalibFileInfo {
	return s.Core.FiveHoleService.GetCalibInfo(probeID)
}

// StartFiveHoleTraversal 启动五孔移位测试（统一控制 1-3 探针）
func (s *FiveHoleService) StartFiveHoleTraversal(config types.FiveHoleTraversalConfig) (string, error) {
	svc := s.Core.FiveHoleService

	// 多设备冲突检查
	if err := five_hole.CheckMotionConflict(config); err != nil {
		return "", err
	}
	if warn := five_hole.CheckDeviceChannelOverlap(config); warn != "" {
		slog.Warn(warn)
	}

	// 启动所有涉及到的采集设备（去重）
	deviceIDs := five_hole.CollectDeviceIDs(config)
	for _, deviceID := range deviceIDs {
		if deviceID == "" {
			continue
		}
		if !s.Core.DeviceManager.IsAcquiring(deviceID) {
			periodMs := 50
			if profile := s.Core.DeviceManager.GetProfileByID(deviceID); profile != nil && profile.PeriodMs > 0 {
				periodMs = profile.PeriodMs
			}
			if err := s.Core.DeviceManager.StartAcquisition(deviceID, periodMs); err != nil {
				return "", fmt.Errorf("启动采集设备 %s 失败: %w", deviceID, err)
			}
			maxWait := periodMs * 20
			if maxWait < 2000 {
				maxWait = 2000
			}
			pollInterval := periodMs
			if pollInterval < 50 {
				pollInterval = 50
			}
			gotFirstFrame := false
			for elapsed := 0; elapsed < maxWait; elapsed += pollInterval {
				if _, ok := s.Core.DeviceManager.GetLatestData(deviceID); ok {
					gotFirstFrame = true
					break
				}
				time.Sleep(time.Duration(pollInterval) * time.Millisecond)
			}
			// 等待超时仍未拿到首帧，提前返回错误（避免测试进入采样阶段才以 ErrDataStagnant 形式失败）
			if !gotFirstFrame {
				return "", fmt.Errorf("采集设备 %s 在 %dms 内未产生数据，请检查设备连接与采集配置", deviceID, maxWait)
			}
		}
	}

	return svc.Start(config)
}

// PauseFiveHoleTraversal 暂停五孔移位测试
func (s *FiveHoleService) PauseFiveHoleTraversal() {
	s.Core.FiveHoleService.Pause()
}

// ResumeFiveHoleTraversal 恢复五孔移位测试
func (s *FiveHoleService) ResumeFiveHoleTraversal() {
	s.Core.FiveHoleService.Resume()
}

// StopFiveHoleTraversal 停止五孔移位测试（对所有涉及到的位移机构急停）
func (s *FiveHoleService) StopFiveHoleTraversal() {
	svc := s.Core.FiveHoleService
	svc.Stop()
	// 对所有涉及到的位移机构急停
	config := svc.GetConfig()
	controllerIDs := make(map[string]bool)
	for _, probe := range config.Probes {
		if !probe.Enabled {
			continue
		}
		if probe.MotionX.ControllerID != "" {
			controllerIDs[probe.MotionX.ControllerID] = true
		}
		if probe.MotionY.ControllerID != "" {
			controllerIDs[probe.MotionY.ControllerID] = true
		}
	}
	for mcID := range controllerIDs {
		if s.Core.MotionManager.IsConnected(mcID) {
			s.Core.EmergencyStopWithRetry(mcID)
		}
	}
}

// StartFiveHoleRealtimeMonitor 启动五孔实时数据监控
func (s *FiveHoleService) StartFiveHoleRealtimeMonitor(config types.FiveHoleTraversalConfig) {
	s.Core.FiveHoleService.StartRealtimeMonitor(config)
}

// StopFiveHoleRealtimeMonitor 停止五孔实时数据监控
func (s *FiveHoleService) StopFiveHoleRealtimeMonitor() {
	s.Core.FiveHoleService.StopRealtimeMonitor()
}

// SelectAndStartFiveHoleRealtimeRecording 弹出保存对话框选择路径后开始五孔实时数据录制
func (s *FiveHoleService) SelectAndStartFiveHoleRealtimeRecording() (string, error) {
	svc := s.Core.FiveHoleService

	dlg := s.Core.App.Dialog.SaveFile()
	dlg.SetOptions(&application.SaveFileDialogOptions{
		Title:    "选择五孔实时数据保存位置",
		Filename: fmt.Sprintf("fivehole-realtime-%s.csv", time.Now().Format("2006-01-02-15-04-05")),
	})
	dlg.AddFilter("CSV文件", "*.csv")
	dlg.AddFilter("所有文件", "*.*")
	filePath, err := dlg.PromptForSingleSelection()
	if err != nil {
		if isDialogCancelled(err) {
			return "", nil // 用户取消对话框，静默返回
		}
		return "", err
	}
	if filePath == "" {
		return "", nil // 用户取消
	}

	if err := svc.StartRealtimeRecording(filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

// StopFiveHoleRealtimeRecording 停止五孔实时数据录制
func (s *FiveHoleService) StopFiveHoleRealtimeRecording() {
	s.Core.FiveHoleService.StopRealtimeRecording()
}

// IsFiveHoleRealtimeRecording 五孔实时数据是否正在录制
func (s *FiveHoleService) IsFiveHoleRealtimeRecording() bool {
	return s.Core.FiveHoleService.IsRealtimeRecording()
}

// GetFiveHoleTraversalStatus 获取五孔移位测试状态
func (s *FiveHoleService) GetFiveHoleTraversalStatus() types.FiveHoleTraversalTaskStatus {
	return s.Core.FiveHoleService.GetStatus()
}

// SelectFiveHoleCalibFiles 选择五孔校准文件（.cal / .prb）
func (s *FiveHoleService) SelectFiveHoleCalibFiles() []string {
	filePaths, err := s.Core.App.Dialog.OpenFile().
		SetTitle("选择五孔校准文件").
		AddFilter("五孔校准数据文件", "*.cal;*.prb").
		AddFilter("所有文件", "*.*").
		PromptForMultipleSelection()
	if err != nil {
		slog.Error("select five hole calib files failed", "err", err)
		return []string{}
	}
	return filePaths
}

// SaveFiveHoleConfig 保存五孔移位测试配置（JSON 文件持久化）
func (s *FiveHoleService) SaveFiveHoleConfig(config types.FiveHoleTraversalConfig) error {
	configDir := s.Core.getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir failed: %w", err)
	}
	filePath := filepath.Join(configDir, "five_hole_config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config failed: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write config file failed: %w", err)
	}
	return nil
}

// LoadFiveHoleConfig 加载五孔移位测试配置
func (s *FiveHoleService) LoadFiveHoleConfig() (types.FiveHoleTraversalConfig, error) {
	configDir := s.Core.getConfigDir()
	filePath := filepath.Join(configDir, "five_hole_config.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return types.FiveHoleTraversalConfig{}, nil // 无配置返回默认
		}
		return types.FiveHoleTraversalConfig{}, fmt.Errorf("read config file failed: %w", err)
	}
	var config types.FiveHoleTraversalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return types.FiveHoleTraversalConfig{}, fmt.Errorf("unmarshal config failed: %w", err)
	}
	return config, nil
}
