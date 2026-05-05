package app

import (
	"fmt"

	"yx-daq/internal/types"
)

// CalibrationService 五孔探针校准服务
type CalibrationService struct {
	Core *Core
}

// StartCalibration 启动校准
func (s *CalibrationService) StartCalibration(config types.CalibrationConfig) (string, error) {
	if s.Core.CalibService == nil {
		return "", fmt.Errorf("calibration service not initialized")
	}
	return s.Core.CalibService.Start(config)
}

// PauseCalibration 暂停校准
func (s *CalibrationService) PauseCalibration() {
	if s.Core.CalibService == nil {
		return
	}
	s.Core.CalibService.Pause()
}

// ResumeCalibration 恢复校准
func (s *CalibrationService) ResumeCalibration() {
	if s.Core.CalibService == nil {
		return
	}
	s.Core.CalibService.Resume()
}

// StopCalibration 停止校准
func (s *CalibrationService) StopCalibration() {
	if s.Core.CalibService == nil {
		return
	}
	s.Core.CalibService.Stop()
}

// GetCalibrationStatus 获取校准状态
func (s *CalibrationService) GetCalibrationStatus() types.CalibrationTaskStatus {
	if s.Core.CalibService == nil {
		return types.CalibrationTaskStatus{}
	}
	return s.Core.CalibService.GetStatus()
}
