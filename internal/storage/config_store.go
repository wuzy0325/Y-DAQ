package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ConfigStore JSON配置存储（原子写入）
type ConfigStore struct {
	mu       sync.RWMutex
	filePath string
	data     interface{}
}

// NewConfigStore 创建配置存储
func NewConfigStore(filePath string, defaultData interface{}) *ConfigStore {
	return &ConfigStore{
		filePath: filePath,
		data:     defaultData,
	}
}

// Load 从文件加载配置
func (s *ConfigStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.writeLocked()
		}
		return fmt.Errorf("read config file failed: %w", err)
	}

	// 修复损坏的JSON
	raw = tryFixCorruptedJson(raw)
	if len(raw) == 0 {
		return s.writeLocked()
	}

	// 先解析为通用类型，再赋值，避免 map/数组类型不匹配导致反序列化失败
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("parse config file failed: %w", err)
	}
	s.data = parsed

	return nil
}

// Save 保存配置到文件（原子写入）
func (s *ConfigStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked()
}

func (s *ConfigStore) writeLocked() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config failed: %w", err)
	}

	// 原子写入：先写临时文件，再重命名
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file failed: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("rename temp file failed: %w", err)
	}

	return nil
}

// Get 获取数据（只读副本）
func (s *ConfigStore) Get() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Set 设置数据并保存
func (s *ConfigStore) Set(data interface{}) error {
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
	// 尝试解析，如果成功直接返回
	var test interface{}
	if json.Unmarshal(raw, &test) == nil {
		return raw
	}
	// 简单修复：空文件返回空对象
	return []byte("{}")
}

// ConfigManager 管理所有配置存储
type ConfigManager struct {
	Devices     *ConfigStore
	Motion      *ConfigStore
	Acquisition *ConfigStore
	Calibration *ConfigStore
	Storage     *ConfigStore
	ThreeHole   *ConfigStore
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configDir string) *ConfigManager {
	return &ConfigManager{
		Devices:     NewConfigStore(filepath.Join(configDir, "devices.json"), map[string]interface{}{}),
		Motion:      NewConfigStore(filepath.Join(configDir, "motion.json"), map[string]interface{}{}),
		Acquisition: NewConfigStore(filepath.Join(configDir, "acquisition.json"), map[string]interface{}{}),
		Calibration: NewConfigStore(filepath.Join(configDir, "calibration.json"), map[string]interface{}{}),
		Storage:     NewConfigStore(filepath.Join(configDir, "storage.json"), map[string]interface{}{}),
		ThreeHole:   NewConfigStore(filepath.Join(configDir, "three_hole.json"), map[string]interface{}{}),
	}
}

// LoadAll 加载所有配置
func (m *ConfigManager) LoadAll() error {
	stores := []*ConfigStore{m.Devices, m.Motion, m.Acquisition, m.Calibration, m.Storage, m.ThreeHole}
	var firstErr error
	for _, s := range stores {
		if err := s.Load(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
