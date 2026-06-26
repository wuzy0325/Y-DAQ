package app

import (
	"fmt"
	"log/slog"
	"time"

	"yx-daq/internal/types"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ThreeHoleService 三孔移位插值测试服务（Wails 绑定层，支持多探针）
type ThreeHoleService struct {
	Core *Core
}

// OpenTestWindow 打开探针测试窗口（复用已有窗口，不重复创建）
func (s *ThreeHoleService) OpenTestWindow(probeID string) string {
	winName := "three-hole-" + probeID
	title := "三孔移位插值测试 - 探针" + string(probeID[len(probeID)-1])

	if existing, ok := s.Core.App.Window.GetByName(winName); ok {
		existing.Focus()
		return "focused"
	}

	win := s.Core.App.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:  winName,
		Title: title,
		Width: 1400, Height: 960,
		MinWidth: 1100, MinHeight: 800,
		BackgroundColour: application.NewRGB(10, 10, 26),
		URL:  "/#/three-hole-test?probe=" + probeID,
	})
	win.Show()
	return "opened"
}

// LoadThreeHoleCalibFiles 加载三孔校准文件
func (s *ThreeHoleService) LoadThreeHoleCalibFiles(probeID string, filePaths []string) error {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return fmt.Errorf("probe %s not initialized", probeID)
	}
	return svc.LoadCalibFiles(filePaths)
}

// IsThreeHoleCalibLoaded 三孔校准文件是否已加载
func (s *ThreeHoleService) IsThreeHoleCalibLoaded(probeID string) bool {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return false
	}
	return svc.IsCalibLoaded()
}

// GetThreeHoleCalibInfo 获取三孔校准文件信息
func (s *ThreeHoleService) GetThreeHoleCalibInfo(probeID string) []types.ThreeHoleCalibFileInfo {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return nil
	}
	return svc.GetCalibInfo()
}

// StartThreeHoleTraversal 启动三孔移位测试
func (s *ThreeHoleService) StartThreeHoleTraversal(probeID string, config types.ThreeHoleTraversalConfig) (string, error) {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return "", fmt.Errorf("probe %s not initialized", probeID)
	}
	if config.DeviceID == "" {
		return "", fmt.Errorf("未选择采集设备")
	}
	if err := s.Core.CheckThreeHoleMotionConflict(probeID, config.MotionControllerID); err != nil {
		return "", err
	}
	if warn := s.Core.CheckThreeHoleDeviceChannelOverlap(probeID, config.DeviceID, config.ProbeChannels); warn != "" {
		slog.Warn(warn)
	}
	if !s.Core.DeviceManager.IsAcquiring(config.DeviceID) {
		periodMs := 50
		if profile := s.Core.DeviceManager.GetProfileByID(config.DeviceID); profile != nil && profile.PeriodMs > 0 {
			periodMs = profile.PeriodMs
		}
		if err := s.Core.DeviceManager.StartAcquisition(config.DeviceID, periodMs); err != nil {
			return "", fmt.Errorf("启动采集失败: %w", err)
		}
		maxWait := periodMs * 20
		if maxWait < 2000 {
			maxWait = 2000
		}
		pollInterval := periodMs
		if pollInterval < 50 {
			pollInterval = 50
		}
		for elapsed := 0; elapsed < maxWait; elapsed += pollInterval {
			if _, ok := s.Core.DeviceManager.GetLatestData(config.DeviceID); ok {
				break
			}
			time.Sleep(time.Duration(pollInterval) * time.Millisecond)
		}
	}
	return svc.Start(config)
}

// PauseThreeHoleTraversal 暂停三孔移位测试
func (s *ThreeHoleService) PauseThreeHoleTraversal(probeID string) {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return
	}
	svc.Pause()
}

// ResumeThreeHoleTraversal 恢复三孔移位测试
func (s *ThreeHoleService) ResumeThreeHoleTraversal(probeID string) {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return
	}
	svc.Resume()
}

// StopThreeHoleTraversal 停止三孔移位测试
func (s *ThreeHoleService) StopThreeHoleTraversal(probeID string) {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return
	}
	svc.Stop()
	config := svc.GetConfig()
	mcID := config.MotionControllerID
	if mcID != "" && s.Core.MotionManager.IsConnected(mcID) {
		s.Core.EmergencyStopWithRetry(mcID)
	} else if mcID == "" {
		slog.Warn("三孔测试停止: 未指定运动控制器ID，跳过急停")
	}
}

// StartThreeHoleRealtimeMonitor 启动三孔实时数据监控
func (s *ThreeHoleService) StartThreeHoleRealtimeMonitor(probeID string, config types.ThreeHoleTraversalConfig) {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return
	}
	svc.StartRealtimeMonitor(config)
}

// StopThreeHoleRealtimeMonitor 停止三孔实时数据监控
func (s *ThreeHoleService) StopThreeHoleRealtimeMonitor(probeID string) {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return
	}
	svc.StopRealtimeMonitor()
}

// SelectAndStartThreeHoleRealtimeRecording 弹出保存对话框选择路径后开始三孔实时数据录制
func (s *ThreeHoleService) SelectAndStartThreeHoleRealtimeRecording(probeID string) (string, error) {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return "", fmt.Errorf("probe %s not initialized", probeID)
	}

	dlg := s.Core.App.Dialog.SaveFile()
	dlg.SetOptions(&application.SaveFileDialogOptions{
		Title:    "选择三孔实时数据保存位置",
		Filename: fmt.Sprintf("threehole-realtime-%s.csv", time.Now().Format("2006-01-02-15-04-05")),
	})
	dlg.AddFilter("CSV文件", "*.csv")
	dlg.AddFilter("所有文件", "*.*")
	filePath, err := dlg.PromptForSingleSelection()
	if err != nil {
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

// StopThreeHoleRealtimeRecording 停止三孔实时数据录制
func (s *ThreeHoleService) StopThreeHoleRealtimeRecording(probeID string) {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return
	}
	svc.StopRealtimeRecording()
}

// IsThreeHoleRealtimeRecording 三孔实时数据是否正在录制
func (s *ThreeHoleService) IsThreeHoleRealtimeRecording(probeID string) bool {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return false
	}
	return svc.IsRealtimeRecording()
}

// GetThreeHoleTraversalStatus 获取三孔移位测试状态
func (s *ThreeHoleService) GetThreeHoleTraversalStatus(probeID string) types.ThreeHoleTraversalTaskStatus {
	svc := s.Core.ThreeHoleServices[probeID]
	if svc == nil {
		return types.ThreeHoleTraversalTaskStatus{}
	}
	return svc.GetStatus()
}

// SelectThreeHoleCalibFiles 选择三孔校准文件
func (s *ThreeHoleService) SelectThreeHoleCalibFiles() []string {
	filePaths, err := s.Core.App.Dialog.OpenFile().
		SetTitle("选择三孔校准文件").
		AddFilter("校准数据文件", "*.dat;*.txt;*.prb").
		AddFilter("所有文件", "*.*").
		PromptForMultipleSelection()
	if err != nil {
		slog.Error("select calib files failed", "err", err)
		return []string{}
	}
	return filePaths
}
