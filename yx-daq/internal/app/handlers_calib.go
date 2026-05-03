package app

import (
	"fmt"

	"yx-daq/internal/types"
)

// ==================== 校准 API ====================

// StartCalibration 启动校准
func (a *App) StartCalibration(config types.CalibrationConfig) (string, error) {
	if a.calibService == nil {
		return "", fmt.Errorf("calibration service not initialized")
	}
	return a.calibService.Start(config)
}

// PauseCalibration 暂停校准
func (a *App) PauseCalibration() {
	if a.calibService == nil {
		return
	}
	a.calibService.Pause()
}

// ResumeCalibration 恢复校准
func (a *App) ResumeCalibration() {
	if a.calibService == nil {
		return
	}
	a.calibService.Resume()
}

// StopCalibration 停止校准
func (a *App) StopCalibration() {
	if a.calibService == nil {
		return
	}
	a.calibService.Stop()
}

// GetCalibrationStatus 获取校准状态
func (a *App) GetCalibrationStatus() types.CalibrationTaskStatus {
	if a.calibService == nil {
		return types.CalibrationTaskStatus{}
	}
	return a.calibService.GetStatus()
}


