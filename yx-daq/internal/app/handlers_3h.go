package app

import (
	"fmt"
	"log/slog"
	"time"

	"yx-daq/internal/types"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ==================== 三孔移位测试 API ====================

// LoadThreeHoleCalibFiles 加载三孔校准文件
func (a *App) LoadThreeHoleCalibFiles(filePaths []string) error {
	if a.threeHoleService == nil {
		return fmt.Errorf("three hole service not initialized")
	}
	return a.threeHoleService.LoadCalibFiles(filePaths)
}

// IsThreeHoleCalibLoaded 三孔校准文件是否已加载
func (a *App) IsThreeHoleCalibLoaded() bool {
	if a.threeHoleService == nil {
		return false
	}
	return a.threeHoleService.IsCalibLoaded()
}

// GetThreeHoleCalibInfo 获取三孔校准文件信息
func (a *App) GetThreeHoleCalibInfo() []types.ThreeHoleCalibFileInfo {
	if a.threeHoleService == nil {
		return nil
	}
	return a.threeHoleService.GetCalibInfo()
}

// StartThreeHoleTraversal 启动三孔移位测试
func (a *App) StartThreeHoleTraversal(config types.ThreeHoleTraversalConfig) (string, error) {
	if a.threeHoleService == nil {
		return "", fmt.Errorf("three hole service not initialized")
	}
	if config.DeviceID == "" {
		return "", fmt.Errorf("未选择采集设备")
	}
	if !a.deviceManager.IsAcquiring(config.DeviceID) {
		periodMs := 50
		if profile := a.deviceManager.GetProfileByID(config.DeviceID); profile != nil && profile.PeriodMs > 0 {
			periodMs = profile.PeriodMs
		}
		if err := a.deviceManager.StartAcquisition(config.DeviceID, periodMs); err != nil {
			return "", fmt.Errorf("启动采集失败: %w", err)
		}
		// 等待采集数据就绪，最多等待采集周期的 20 倍
		maxWait := periodMs * 20
		if maxWait < 2000 {
			maxWait = 2000
		}
		pollInterval := periodMs
		if pollInterval < 50 {
			pollInterval = 50
		}
		for elapsed := 0; elapsed < maxWait; elapsed += pollInterval {
			if _, ok := a.deviceManager.GetLatestData(config.DeviceID); ok {
				break
			}
			time.Sleep(time.Duration(pollInterval) * time.Millisecond)
		}
	}
	return a.threeHoleService.Start(config)
}

// PauseThreeHoleTraversal 暂停三孔移位测试
func (a *App) PauseThreeHoleTraversal() {
	if a.threeHoleService == nil {
		return
	}
	a.threeHoleService.Pause()
}

// ResumeThreeHoleTraversal 恢复三孔移位测试
func (a *App) ResumeThreeHoleTraversal() {
	if a.threeHoleService == nil {
		return
	}
	a.threeHoleService.Resume()
}

// StopThreeHoleTraversal 停止三孔移位测试
func (a *App) StopThreeHoleTraversal() {
	if a.threeHoleService == nil {
		return
	}
	a.threeHoleService.Stop()
	config := a.threeHoleService.GetConfig()
	mcID := config.MotionControllerID
	if mcID != "" {
		if a.motionManager.IsConnected(mcID) {
			a.emergencyStopWithRetry(mcID)
		}
	} else {
		slog.Warn("三孔测试停止: 未指定运动控制器ID，将急停所有已连接控制器")
		profiles := a.motionManager.GetProfiles()
		for _, p := range profiles {
			if a.motionManager.IsConnected(p.ID) {
				a.emergencyStopWithRetry(p.ID)
			}
		}
	}
}

// emergencyStopWithRetry 急停运动控制器（重试1次）
func (a *App) emergencyStopWithRetry(mcID string) {
	if err := a.motionManager.EmergencyStop(mcID); err != nil {
		slog.Warn("急停失败，重试1次", "mcID", mcID, "err", err)
		time.Sleep(100 * time.Millisecond)
		if err2 := a.motionManager.EmergencyStop(mcID); err2 != nil {
			slog.Error("急停重试仍失败", "mcID", mcID, "err", err2)
		}
	}
}

// StartThreeHoleRealtimeMonitor 启动三孔实时数据监控
func (a *App) StartThreeHoleRealtimeMonitor(config types.ThreeHoleTraversalConfig) {
	if a.threeHoleService == nil {
		return
	}
	a.threeHoleService.StartRealtimeMonitor(config)
}

// StopThreeHoleRealtimeMonitor 停止三孔实时数据监控
func (a *App) StopThreeHoleRealtimeMonitor() {
	if a.threeHoleService == nil {
		return
	}
	a.threeHoleService.StopRealtimeMonitor()
}

// GetThreeHoleTraversalStatus 获取三孔移位测试状态
func (a *App) GetThreeHoleTraversalStatus() types.ThreeHoleTraversalTaskStatus {
	if a.threeHoleService == nil {
		return types.ThreeHoleTraversalTaskStatus{}
	}
	return a.threeHoleService.GetStatus()
}

// SelectThreeHoleCalibFiles 选择三孔校准文件（弹出文件对话框）
func (a *App) SelectThreeHoleCalibFiles() []string {
	filePaths, err := wailsRuntime.OpenMultipleFilesDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择三孔校准文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "校准数据文件", Pattern: "*.dat;*.txt;*.prb"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		slog.Error("select calib files failed", "err", err)
		return []string{}
	}
	return filePaths
}


