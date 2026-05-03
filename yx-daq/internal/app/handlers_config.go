package app

import (
	"fmt"

	"yx-daq/internal/types"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ==================== 配置持久化 API ====================

// SaveThreeHoleConfig 保存三孔移位测试配置
func (a *App) SaveThreeHoleConfig(config types.ThreeHoleTraversalConfig) error {
	if a.configManager == nil {
		return fmt.Errorf("config manager not initialized")
	}
	return a.configManager.ThreeHole.Set(config)
}

// LoadThreeHoleConfig 加载三孔移位测试配置
func (a *App) LoadThreeHoleConfig() (types.ThreeHoleTraversalConfig, error) {
	if a.configManager == nil {
		return types.ThreeHoleTraversalConfig{}, fmt.Errorf("config manager not initialized")
	}
	return a.configManager.ThreeHole.Get(), nil
}

// ==================== 路径配置 API ====================

// ==================== 路径配置 API ====================

// SetDataSavePath 设置数据保存路径
func (a *App) SetDataSavePath(path string) error {
	if a.configManager == nil {
		return fmt.Errorf("config manager not initialized")
	}
	cfg := a.configManager.Storage.Get()
	cfg.DataSavePath = path
	return a.configManager.Storage.Set(cfg)
}

// SelectDataSavePath 弹出文件夹选择对话框选择数据保存路径
func (a *App) SelectDataSavePath() (string, error) {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择数据保存路径",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}
