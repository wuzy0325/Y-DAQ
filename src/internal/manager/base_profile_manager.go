package manager

import (
	"log/slog"
	"sync"

	"yx-daq/internal/storage"
)

// BaseProfileManager 通用的 profile CRUD 与持久化能力基类
// T 为 profile 类型（如 types.DeviceProfile / types.MotionControllerProfile）
// 子类（DeviceManager / MotionControllerManager）通过嵌入复用通用方法，
// 各自维护 instances / runtimeStatus / onStatusChange 等类型相关字段
type BaseProfileManager[T any] struct {
	sync.RWMutex
	profiles    map[string]T
	configStore *storage.ConfigStore[[]T]
}

// initProfiles 初始化 profiles map（子类构造函数中必须调用）
func (b *BaseProfileManager[T]) initProfiles() {
	if b.profiles == nil {
		b.profiles = make(map[string]T)
	}
}

// SetConfigStore 设置配置存储（用于持久化）
func (b *BaseProfileManager[T]) SetConfigStore(store *storage.ConfigStore[[]T]) {
	b.configStore = store
}

// saveProfilesWithLog 持久化 profiles 到磁盘
// tag 用于错误日志中区分设备/控制器
func (b *BaseProfileManager[T]) saveProfilesWithLog(tag string) {
	if b.configStore == nil {
		return
	}
	if err := b.configStore.Set(b.GetProfiles()); err != nil {
		slog.Error("save profiles failed", "tag", tag, "err", err)
	}
}

// addProfile 添加 profile（不触发持久化，调用方需显式调用 saveProfilesWithLog）
// 因 Go 泛型不支持字段约束，调用方需显式提供 id
func (b *BaseProfileManager[T]) addProfile(id string, profile T) {
	b.Lock()
	defer b.Unlock()
	b.profiles[id] = profile
}

// getProfile 根据 ID 获取 profile 副本
func (b *BaseProfileManager[T]) getProfile(id string) (T, bool) {
	b.RLock()
	defer b.RUnlock()
	p, ok := b.profiles[id]
	return p, ok
}

// GetProfiles 获取所有 profile（切片形式，副本）
func (b *BaseProfileManager[T]) GetProfiles() []T {
	b.RLock()
	defer b.RUnlock()
	result := make([]T, 0, len(b.profiles))
	for _, p := range b.profiles {
		result = append(result, p)
	}
	return result
}
