package main

import (
	"fmt"
	"os"
	"path/filepath"

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

// GetDataDir 获取数据存储目录
func (a *App) GetDataDir() string {
	if a.configManager != nil {
		data := a.configManager.Storage.Get()
		if path, ok := data["dataSavePath"].(string); ok && path != "" {
			return path
		}
	}
	return filepath.Join(a.getConfigDir(), "data")
}

// SetDataSavePath 设置数据保存路径
func (a *App) SetDataSavePath(path string) error {
	if a.configManager == nil {
		return fmt.Errorf("config manager not initialized")
	}
	data := a.configManager.Storage.Get()
	data["dataSavePath"] = path
	return a.configManager.Storage.Set(data)
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

// getConfigDir 获取配置目录
func (a *App) getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "." // 无HOME则用当前目录
	}
	configDir := filepath.Join(home, ".yx-daq")
	os.MkdirAll(configDir, 0755) // ignore error: 目录已存在或后续文件操作会报错
	return configDir
}
