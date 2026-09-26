package types

import "strings"

// LoggingConfig 日志系统配置
type LoggingConfig struct {
	Level          string `json:"level,omitempty"`         // debug/info/warn/error
	Console        bool   `json:"console"`                 // 同时输出到标准错误
	CommEnabled    bool   `json:"commEnabled"`             // 记录设备通信错误
	FrontendErrors bool   `json:"frontendErrors"`          // 收集前端 JS 错误
	RetentionDays  int    `json:"retentionDays,omitempty"` // 日志保留天数
	MaxFileSizeMB  int    `json:"maxFileSizeMb,omitempty"` // 单文件大小上限（MB），超出滚动
}

// LogFileInfo 日志文件信息（供前端日志查看器展示）
type LogFileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"` // Unix 毫秒
	Category string `json:"category"` // app/comm/crash/other
}

// DefaultLoggingConfig 日志系统默认配置
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:          "info",
		Console:        true,
		CommEnabled:    true,
		FrontendErrors: true,
		RetentionDays:  30,
		MaxFileSizeMB:  50,
	}
}

// Normalize 返回补全缺失/非法字段后的配置副本
func (c LoggingConfig) Normalize() LoggingConfig {
	def := DefaultLoggingConfig()
	c.Level = strings.ToLower(strings.TrimSpace(c.Level))
	if c.Level == "" {
		c.Level = def.Level
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = def.RetentionDays
	}
	if c.MaxFileSizeMB <= 0 {
		c.MaxFileSizeMB = def.MaxFileSizeMB
	}
	return c
}

// CategoryFromLogName 从日志文件名解析分类（app/comm/crash/other）
func CategoryFromLogName(name string) string {
	switch {
	case strings.HasPrefix(name, "yx-daq-"):
		return "app"
	case strings.HasPrefix(name, "comm-"):
		return "comm"
	case strings.HasPrefix(name, "crash-"):
		return "crash"
	default:
		return "other"
	}
}
