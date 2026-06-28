package app

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// CoreService 应用生命周期管理服务
type CoreService struct {
	Core *Core
}

// ServiceStartup 在服务注册时调用，初始化核心
func (s *CoreService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.Core.Startup(application.Get())
	return nil
}

// ServiceShutdown 在应用关闭时调用
func (s *CoreService) ServiceShutdown() error {
	s.Core.Shutdown()
	return nil
}
