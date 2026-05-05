package app

import (
	"fmt"

	"yx-daq/internal/types"
)

// ConfigService 配置持久化服务
type ConfigService struct {
	Core *Core
}

// SaveThreeHoleConfig 保存三孔移位测试配置（探针1）
func (s *ConfigService) SaveThreeHoleProbe1Config(config types.ThreeHoleTraversalConfig) error {
	if s.Core.ConfigManager == nil {
		return fmt.Errorf("config manager not initialized")
	}
	return s.Core.ConfigManager.ThreeHoleProbe1.Set(config)
}

// LoadThreeHoleConfig 加载三孔移位测试配置（探针1）
func (s *ConfigService) LoadThreeHoleProbe1Config() (types.ThreeHoleTraversalConfig, error) {
	if s.Core.ConfigManager == nil {
		return types.ThreeHoleTraversalConfig{}, fmt.Errorf("config manager not initialized")
	}
	return s.Core.ConfigManager.ThreeHoleProbe1.Get(), nil
}

// SaveThreeHoleProbe2Config 保存三孔移位测试配置（探针2）
func (s *ConfigService) SaveThreeHoleProbe2Config(config types.ThreeHoleTraversalConfig) error {
	if s.Core.ConfigManager == nil {
		return fmt.Errorf("config manager not initialized")
	}
	return s.Core.ConfigManager.ThreeHoleProbe2.Set(config)
}

// LoadThreeHoleProbe2Config 加载三孔移位测试配置（探针2）
func (s *ConfigService) LoadThreeHoleProbe2Config() (types.ThreeHoleTraversalConfig, error) {
	if s.Core.ConfigManager == nil {
		return types.ThreeHoleTraversalConfig{}, fmt.Errorf("config manager not initialized")
	}
	return s.Core.ConfigManager.ThreeHoleProbe2.Get(), nil
}

// SetDataSavePath 设置数据保存路径
func (s *ConfigService) SetDataSavePath(path string) error {
	if s.Core.ConfigManager == nil {
		return fmt.Errorf("config manager not initialized")
	}
	cfg := s.Core.ConfigManager.Storage.Get()
	cfg.DataSavePath = path
	return s.Core.ConfigManager.Storage.Set(cfg)
}

// SelectDataSavePath 弹出文件夹选择对话框选择数据保存路径
func (s *ConfigService) SelectDataSavePath() (string, error) {
	dir, err := s.Core.App.Dialog.OpenFile().
		SetTitle("选择数据保存路径").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return dir, nil
}
