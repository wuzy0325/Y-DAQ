package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"yx-daq/internal/types"
)

// ConfigStore JSON配置存储（原子写入）
type ConfigStore[T any] struct {
	mu       sync.RWMutex
	filePath string
	data     T
}

// NewConfigStore 创建配置存储
func NewConfigStore[T any](filePath string, defaultData T) *ConfigStore[T] {
	return &ConfigStore[T]{
		filePath: filePath,
		data:     defaultData,
	}
}

// Load 从文件加载配置
func (s *ConfigStore[T]) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.writeLocked()
		}
		return fmt.Errorf("read config file failed: %w", err)
	}

	raw = tryFixCorruptedJson(raw)
	if len(raw) == 0 {
		return s.writeLocked()
	}

	var parsed T
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return s.writeLocked()
	}
	s.data = parsed

	return nil
}

// Save 保存配置到文件（原子写入）
func (s *ConfigStore[T]) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked()
}

func (s *ConfigStore[T]) writeLocked() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config failed: %w", err)
	}

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file failed: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("rename temp file failed: %w", err)
	}

	return nil
}

// Get 获取数据
func (s *ConfigStore[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Set 设置数据并保存
func (s *ConfigStore[T]) Set(data T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	return s.writeLocked()
}

// tryFixCorruptedJson 尝试修复损坏的JSON
func tryFixCorruptedJson(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	var test any
	if json.Unmarshal(raw, &test) == nil {
		return raw
	}
	return []byte("{}")
}

// ConfigManager 管理所有配置存储
type ConfigManager struct {
	Devices     *ConfigStore[[]types.DeviceProfile]
	Motion      *ConfigStore[[]types.MotionControllerProfile]
	Acquisition *ConfigStore[types.AcquisitionConfig]
	Calibration *ConfigStore[types.CalibrationConfig] // 五孔校准配置
	Storage     *ConfigStore[types.StorageConfig]
	ThreeHole   *ConfigStore[types.ThreeHoleTraversalConfig]
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configDir string) *ConfigManager {
	return &ConfigManager{
		Devices:     NewConfigStore(filepath.Join(configDir, "devices.json"), []types.DeviceProfile{}),
		Motion:      NewConfigStore(filepath.Join(configDir, "motion.json"), []types.MotionControllerProfile{}),
		Acquisition: NewConfigStore(filepath.Join(configDir, "acquisition.json"), types.AcquisitionConfig{}),
		Calibration: NewConfigStore(filepath.Join(configDir, "calibration.json"), types.CalibrationConfig{}),
		Storage:     NewConfigStore(filepath.Join(configDir, "storage.json"), types.StorageConfig{}),
		ThreeHole:   NewConfigStore(filepath.Join(configDir, "three_hole.json"), types.ThreeHoleTraversalConfig{}),
	}
}

// LoadAll 加载所有配置
func (m *ConfigManager) LoadAll() error {
	var firstErr error
	if err := m.Devices.Load(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := m.Motion.Load(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := m.Acquisition.Load(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := m.Calibration.Load(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := m.Storage.Load(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := m.ThreeHole.Load(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
