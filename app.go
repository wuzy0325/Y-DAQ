package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/calibration"
	"yx-daq/internal/manager"
	"yx-daq/internal/scanner"
	"yx-daq/internal/storage"
	"yx-daq/internal/three_hole"
	"yx-daq/internal/types"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 主应用结构
type App struct {
	ctx                context.Context
	deviceManager      *manager.DeviceManager
	motionManager      *manager.MotionControllerManager
	acquisitionHub     *manager.AcquisitionHub
	calibService       *calibration.CalibrationService
	threeHoleService   *three_hole.ThreeHoleTraversalService
	configManager      *storage.ConfigManager
	dataStorage        *storage.DataStorageService
	pdfReport          *storage.PdfReportService
	daqScanner         *scanner.XYDAQ16Scanner
	publishCancel      chan struct{}
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{
		deviceManager:  manager.NewDeviceManager(),
		motionManager:  manager.NewMotionControllerManager(),
		acquisitionHub: manager.NewAcquisitionHub(),
		pdfReport:      storage.NewPdfReportService(),
		daqScanner:     scanner.NewXYDAQ16Scanner(),
		publishCancel:  make(chan struct{}),
	}
}

// startup 应用启动
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 初始化配置
	configDir := a.getConfigDir()
	a.configManager = storage.NewConfigManager(configDir)
	if err := a.configManager.LoadAll(); err != nil {
		log.Printf("load config failed: %v", err)
	}

	// 设置数据管道: 设备数据 → AcquisitionHub + DataStorage
	a.dataStorage = storage.NewDataStorageService(a.GetDataDir())
	a.deviceManager.SetDataSink(func(payload types.DataPayload) {
		a.acquisitionHub.OnData(payload)
		if a.dataStorage.IsRecording() {
			a.dataStorage.HandlePayload(payload)
		}
	})

	// 设置设备配置持久化
	a.deviceManager.SetConfigStore(a.configManager.Devices)

	// 初始化设备管理器
	a.deviceManager.Init()

	// 初始化运动控制器管理器（从配置文件加载）
	a.initMotionFromConfig()

	// 初始化校准服务
	a.calibService = calibration.NewCalibrationService(&wailsEventPublisher{app: a})
	a.calibService.SetDataGetter(func(deviceID string, channelIndex int) (float64, bool) {
		return a.deviceManager.GetChannelValue(deviceID, channelIndex)
	})
	a.calibService.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			if !ch.Enabled {
				continue
			}
			// 从校准配置的设备ID获取数据
			val, ok := a.acquisitionHub.GetLatestValue(a.calibService.GetStatus().TaskID, ch.Channel)
			if ok {
				result[ch.Channel] = val
			}
		}
		return result, nil
	})
	a.calibService.SetMotionController(func(axis types.AxisName, position float64) error {
		// 使用第一个已连接的运动控制器
		profiles := a.motionManager.GetProfiles()
		for _, p := range profiles {
			if a.motionManager.IsConnected(p.ID) {
				return a.motionManager.MoveTo(p.ID, axis, position)
			}
		}
		return fmt.Errorf("no motion controller connected")
	})

	// 初始化三孔移位测试服务
	a.threeHoleService = three_hole.NewThreeHoleTraversalService(&threeHoleEventPublisher{app: a})
	a.threeHoleService.SetBatchGetter(func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			if !ch.Enabled {
				continue
			}
			// 从所有已连接的采集设备获取数据
			snapshots := a.acquisitionHub.GetSnapshot()
			for _, snap := range snapshots {
				for i, idx := range snap.ChannelIndices {
					if idx == ch.Channel && i < len(snap.Channels) {
						result[ch.Channel] = snap.Channels[i]
						break
					}
				}
				if _, ok := result[ch.Channel]; ok {
					break
				}
			}
		}
		return result, nil
	})
	a.threeHoleService.SetMotionController(func(axis types.AxisName, position float64) error {
		profiles := a.motionManager.GetProfiles()
		for _, p := range profiles {
			if a.motionManager.IsConnected(p.ID) {
				return a.motionManager.MoveTo(p.ID, axis, position)
			}
		}
		return fmt.Errorf("no motion controller connected")
	})

	// 启动数据快照发布
	a.acquisitionHub.SetOnSnapshot(func(snapshots []types.DataPayload) {
		wailsRuntime.EventsEmit(a.ctx, "daq:data-snapshot", snapshots)
	})
	go a.acquisitionHub.StartPublishing(a.publishCancel)

	// 启动运动状态轮询广播
	go a.broadcastMotionStatus()

	log.Println("YX-DAQ application started")
}

// shutdown 应用关闭
func (a *App) shutdown(ctx context.Context) {
	a.publishCancel <- struct{}{}
	a.dataStorage.StopRecording()
	a.deviceManager.StopAcquisitionAll()
	// 保存设备配置
	if a.configManager != nil {
		profiles := a.deviceManager.GetProfiles()
		if err := a.configManager.Devices.Set(profiles); err != nil {
			log.Printf("save device profiles on shutdown failed: %v", err)
		}
		// 保存运动控制器配置
		motionProfiles := a.motionManager.GetProfiles()
		if err := a.configManager.Motion.Set(motionProfiles); err != nil {
			log.Printf("save motion profiles on shutdown failed: %v", err)
		}
	}
	log.Println("YX-DAQ application shutdown")
}

// getConfigDir 获取配置目录
func (a *App) getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	configDir := filepath.Join(home, ".yx-daq")
	os.MkdirAll(configDir, 0755)
	return configDir
}

// initMotionFromConfig 从配置文件初始化运动控制器
func (a *App) initMotionFromConfig() {
	// 尝试从配置文件加载已保存的 profiles
	if a.configManager != nil {
		data := a.configManager.Motion.Get()
		if data != nil {
			// 将 interface{} 序列化再反序列化为 []MotionControllerProfile
			jsonBytes, err := json.Marshal(data)
			if err == nil {
				var savedProfiles []types.MotionControllerProfile
				if json.Unmarshal(jsonBytes, &savedProfiles) == nil && len(savedProfiles) > 0 {
					for _, p := range savedProfiles {
						a.motionManager.AddProfile(p)
					}
					log.Printf("loaded %d motion controller profiles from config", len(savedProfiles))

					// 确保B140 profile存在（配置文件中可能没有）
					profiles := a.motionManager.GetProfiles()
					hasB140 := false
					for _, p := range profiles {
						if p.Type == types.MotionTypeB140 {
							hasB140 = true
							break
						}
					}
					if !hasB140 {
						defaultAxes := []types.AxisConfig{
							{Name: types.AxisX, Enabled: true, Kind: types.AxisKindLinear, Inverted: false, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5, MaxSpeed: 50, EncoderScale: 0.005},
							{Name: types.AxisY, Enabled: true, Kind: types.AxisKindLinear, Inverted: false, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5, MaxSpeed: 50, EncoderScale: 0.005},
							{Name: types.AxisZ, Enabled: true, Kind: types.AxisKindLinear, Inverted: false, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5, MaxSpeed: 50, EncoderScale: 0.005},
							{Name: types.AxisU, Enabled: true, Kind: types.AxisKindRotary, Inverted: false, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 4, MaxSpeed: 30, EncoderScale: 0.005},
						}
						b140Profile := types.MotionControllerProfile{
							ID:        "b140-mc-1",
							Name:      "B140 运动控制器",
							Type:      types.MotionTypeB140,
							Address:   "192.168.3.100",
							Port:      23,
							TimeoutMs: 5000,
							Axes:      defaultAxes,
						}
						a.motionManager.AddProfile(b140Profile)
						log.Printf("added default B140 motion controller profile")
					}

					// 启动状态轮询
					go a.motionManager.StartPolling()
					// 自动连接模拟控制器
					for _, p := range a.motionManager.GetProfiles() {
						if p.Type == types.MotionTypeSimulated {
							if err := a.motionManager.Connect(p.ID); err != nil {
								log.Printf("auto-connect simulated motion controller failed: %v", err)
							}
						}
					}
					return
				}
			}
		}
	}

	// 无已保存配置，使用默认初始化
	a.motionManager.Init()
}

// saveMotionConfig 保存运动控制器配置到文件
func (a *App) saveMotionConfig() {
	if a.configManager == nil {
		return
	}
	profiles := a.motionManager.GetProfiles()
	if err := a.configManager.Motion.Set(profiles); err != nil {
		log.Printf("save motion config failed: %v", err)
	}
}

// broadcastMotionStatus 广播运动状态
func (a *App) broadcastMotionStatus() {
	ticker := time.NewTicker(time.Duration(types.MotionPollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-a.publishCancel:
			return
		case <-ticker.C:
			statuses := a.motionManager.GetStatusAll()
			wailsRuntime.EventsEmit(a.ctx, "motion:status-updated", statuses)
		}
	}
}

// ==================== 设备管理 API ====================

// GetDeviceProfiles 获取所有设备配置
func (a *App) GetDeviceProfiles() []types.DeviceProfile {
	return a.deviceManager.GetProfiles()
}

// AddDeviceProfile 添加设备配置
func (a *App) AddDeviceProfile(profile types.DeviceProfile) {
	a.deviceManager.AddProfile(profile)
}

// UpdateDeviceProfile 更新设备配置
func (a *App) UpdateDeviceProfile(profile types.DeviceProfile) {
	a.deviceManager.UpdateProfile(profile)
}

// RemoveDeviceProfile 删除设备配置
func (a *App) RemoveDeviceProfile(id string) {
	a.deviceManager.RemoveProfile(id)
}

// ConnectDevice 连接设备
func (a *App) ConnectDevice(id string) error {
	return a.deviceManager.Connect(id)
}

// DisconnectDevice 断开设备
func (a *App) DisconnectDevice(id string) {
	a.deviceManager.Disconnect(id)
}

// StartAcquisition 启动采集
func (a *App) StartAcquisition(id string) error {
	// 从profile中读取采集周期
	periodMs := 50 // 默认50ms
	profile := a.deviceManager.GetProfileByID(id)
	if profile != nil && profile.PeriodMs > 0 {
		periodMs = profile.PeriodMs
	}
	return a.deviceManager.StartAcquisition(id, periodMs)
}

// StopAcquisition 停止采集
func (a *App) StopAcquisition(id string) error {
	return a.deviceManager.StopAcquisition(id)
}

// StartAcquisitionAll 批量启动采集
func (a *App) StartAcquisitionAll() int {
	// 使用默认50ms，因为批量启动无法为每个设备指定不同周期
	return a.deviceManager.StartAcquisitionAll(50)
}

// StopAcquisitionAll 批量停止采集
func (a *App) StopAcquisitionAll() {
	a.deviceManager.StopAcquisitionAll()
}

// GetDeviceStatusAll 获取所有设备状态
func (a *App) GetDeviceStatusAll() []types.DeviceStatus {
	return a.deviceManager.GetStatusAll()
}

// ScanDevices 扫描设备
func (a *App) ScanDevices() []types.DiscoveredDevice {
	devices, err := a.daqScanner.Scan(3000)
	if err != nil {
		log.Printf("scan devices failed: %v", err)
		return []types.DiscoveredDevice{}
	}
	return devices
}

// GetLatestData 获取最新数据快照
func (a *App) GetLatestData() []types.DataPayload {
	return a.acquisitionHub.GetSnapshot()
}

// ==================== 运动控制 API ====================

// GetMotionProfiles 获取所有运动控制器配置
func (a *App) GetMotionProfiles() []types.MotionControllerProfile {
	return a.motionManager.GetProfiles()
}

// AddMotionProfile 添加运动控制器配置
func (a *App) AddMotionProfile(profile types.MotionControllerProfile) {
	a.motionManager.AddProfile(profile)
	a.saveMotionConfig()
}

// ConnectMotion 连接运动控制器
func (a *App) ConnectMotion(id string) error {
	return a.motionManager.Connect(id)
}

// DisconnectMotion 断开运动控制器
func (a *App) DisconnectMotion(id string) {
	a.motionManager.Disconnect(id)
}

// MotionMoveTo 绝对定位
func (a *App) MotionMoveTo(id string, axis types.AxisName, position float64) error {
	return a.motionManager.MoveTo(id, axis, position)
}

// MotionMoveBy 相对移动
func (a *App) MotionMoveBy(id string, axis types.AxisName, delta float64) error {
	return a.motionManager.MoveBy(id, axis, delta)
}

// MotionJog 点动
func (a *App) MotionJog(id string, axis types.AxisName, direction int, speed float64) error {
	return a.motionManager.Jog(id, axis, direction, speed)
}

// MotionHome 回零
func (a *App) MotionHome(id string, axis types.AxisName) error {
	return a.motionManager.Home(id, axis)
}

// MotionStop 停止
func (a *App) MotionStop(id string, axis types.AxisName) error {
	return a.motionManager.Stop(id, axis)
}

// MotionEmergencyStop 急停
func (a *App) MotionEmergencyStop(id string) error {
	return a.motionManager.EmergencyStop(id)
}

// MotionDefinePosition 置位
func (a *App) MotionDefinePosition(id string, axis types.AxisName, position float64) error {
	return a.motionManager.DefinePosition(id, axis, position)
}

// GetMotionStatusAll 获取所有运动控制器状态
func (a *App) GetMotionStatusAll() []types.MotionControllerStatus {
	return a.motionManager.GetStatusAll()
}

// MotionSetAcceleration 设置加速度
func (a *App) MotionSetAcceleration(id string, axis types.AxisName, accel float64) error {
	return a.motionManager.SetAcceleration(id, axis, accel)
}

// MotionSetDeceleration 设置减速度
func (a *App) MotionSetDeceleration(id string, axis types.AxisName, decel float64) error {
	return a.motionManager.SetDeceleration(id, axis, decel)
}

// MotionIsMoving 查询是否有轴在运动
func (a *App) MotionIsMoving(id string) (bool, error) {
	return a.motionManager.IsMoving(id)
}

// MotionIsAxisMoving 查询单轴是否在运动
func (a *App) MotionIsAxisMoving(id string, axis types.AxisName) (bool, error) {
	return a.motionManager.IsAxisMoving(id, axis)
}

// MotionGetLimitStatus 查询轴限位状态
func (a *App) MotionGetLimitStatus(id string, axis types.AxisName) (types.LimitStatus, error) {
	return a.motionManager.GetLimitStatus(id, axis)
}

// MotionWaitForComplete 等待运动完成
func (a *App) MotionWaitForComplete(id string, axis types.AxisName, timeoutMs int) error {
	return a.motionManager.WaitForMotionComplete(id, axis, timeoutMs)
}

// MotionMotorOff 关闭电机
func (a *App) MotionMotorOff(id string) error {
	return a.motionManager.MotorOff(id)
}

// MotionSetAxisDirection 设置轴方向
func (a *App) MotionSetAxisDirection(id string, axis types.AxisName, reverse bool) error {
	return a.motionManager.SetAxisDirection(id, axis, reverse)
}

// UpdateMotionProfile 更新运动控制器配置
func (a *App) UpdateMotionProfile(profile types.MotionControllerProfile) {
	a.motionManager.UpdateProfile(profile)
	a.saveMotionConfig()
}

// RemoveMotionProfile 删除运动控制器配置
func (a *App) RemoveMotionProfile(id string) {
	a.motionManager.RemoveProfile(id)
	a.saveMotionConfig()
}

// ==================== 校准 API ====================

// StartCalibration 启动校准
func (a *App) StartCalibration(config types.CalibrationConfig) (string, error) {
	return a.calibService.Start(config)
}

// PauseCalibration 暂停校准
func (a *App) PauseCalibration() {
	a.calibService.Pause()
}

// ResumeCalibration 恢复校准
func (a *App) ResumeCalibration() {
	a.calibService.Resume()
}

// StopCalibration 停止校准
func (a *App) StopCalibration() {
	a.calibService.Stop()
}

// GetCalibrationStatus 获取校准状态
func (a *App) GetCalibrationStatus() types.CalibrationTaskStatus {
	return a.calibService.GetStatus()
}

// ==================== 数据发布 API ====================

// SetPublishRate 设置数据发布频率
func (a *App) SetPublishRate(hz int) {
	a.acquisitionHub.SetPublishHz(hz)
}

// GetPublishRate 获取数据发布频率
func (a *App) GetPublishRate() int {
	return a.acquisitionHub.GetPublishHz()
}

// ==================== 录制 API ====================

// StartRecording 开始录制
func (a *App) StartRecording() error {
	// 录制前更新输出目录（用户可能修改了保存路径）
	a.dataStorage.SetOutputDir(a.GetDataDir())
	return a.dataStorage.StartRecording()
}

// StopRecording 停止录制
func (a *App) StopRecording() {
	a.dataStorage.StopRecording()
}

// IsRecording 是否正在录制
func (a *App) IsRecording() bool {
	return a.dataStorage.IsRecording()
}

// ==================== Wails事件发布器 ====================

// wailsEventPublisher 通过Wails Events推送校准事件
type wailsEventPublisher struct {
	app *App
}

func (p *wailsEventPublisher) EmitProgress(event types.CalibrationProgressEvent) {
	wailsRuntime.EventsEmit(p.app.ctx, "calibration:progress", event)
}

func (p *wailsEventPublisher) EmitRealtime(event types.CalibrationRealtimeEvent) {
	wailsRuntime.EventsEmit(p.app.ctx, "calibration:realtime", event)
}

func (p *wailsEventPublisher) EmitComplete(event types.CalibrationCompleteEvent) {
	wailsRuntime.EventsEmit(p.app.ctx, "calibration:complete", event)
}

// ==================== 报告导出 API ====================

// ExportCalibrationPDF 导出校准PDF报告
func (a *App) ExportCalibrationPDF() error {
	status := a.calibService.GetStatus()
	if len(status.DataPoints) == 0 {
		return fmt.Errorf("no calibration data to export")
	}

	// 弹出保存文件对话框
	filePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename: fmt.Sprintf("calibration-report-%s.pdf", time.Now().Format("2006-01-02-15-04-05")),
		Title:           "导出校准PDF报告",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "PDF文件", Pattern: "*.pdf"},
		},
	})
	if err != nil {
		return err
	}
	if filePath == "" {
		return nil // 用户取消
	}

	// 获取校准配置 (从当前状态重建)
	config := types.CalibrationConfig{
		Type:            types.CalibrationTypeFiveHole,
		DeviceID:        status.TaskID,
		AlphaAxis:       "X",
		BetaAxis:        "Y",
		DwellTimeMs:     500,
		SamplesPerPoint: 10,
	}

	return a.pdfReport.ExportCalibrationReport(status.DataPoints, config, filePath)
}

// ==================== 数据回放 API ====================

// LoadCSVFile 加载CSV文件用于回放
func (a *App) LoadCSVFile() (string, error) {
	filePath, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择CSV数据文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "CSV文件", Pattern: "*.csv"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}

	return string(data), nil
}

// GetDataDir 获取数据存储目录
func (a *App) GetDataDir() string {
	// 优先从 storage 配置中读取自定义路径
	if a.configManager != nil {
		data := a.configManager.Storage.Get()
		if m, ok := data.(map[string]interface{}); ok {
			if path, ok := m["dataSavePath"].(string); ok && path != "" {
				return path
			}
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
	m, ok := data.(map[string]interface{})
	if !ok {
		m = map[string]interface{}{}
	}
	m["dataSavePath"] = path
	return a.configManager.Storage.Set(m)
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

// ListRecordingFiles 列出录制文件
func (a *App) ListRecordingFiles() []string {
	dataDir := a.GetDataDir()
	os.MkdirAll(dataDir, 0755)

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return []string{}
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".csv" {
			files = append(files, entry.Name())
		}
	}
	return files
}

// ==================== 三孔移位测试 API ====================

// LoadThreeHoleCalibFiles 加载三孔校准文件
func (a *App) LoadThreeHoleCalibFiles(filePaths []string) error {
	return a.threeHoleService.LoadCalibFiles(filePaths)
}

// IsThreeHoleCalibLoaded 三孔校准文件是否已加载
func (a *App) IsThreeHoleCalibLoaded() bool {
	return a.threeHoleService.IsCalibLoaded()
}

// StartThreeHoleTraversal 启动三孔移位测试
func (a *App) StartThreeHoleTraversal(config types.ThreeHoleTraversalConfig) (string, error) {
	return a.threeHoleService.Start(config)
}

// PauseThreeHoleTraversal 暂停三孔移位测试
func (a *App) PauseThreeHoleTraversal() {
	a.threeHoleService.Pause()
}

// ResumeThreeHoleTraversal 恢复三孔移位测试
func (a *App) ResumeThreeHoleTraversal() {
	a.threeHoleService.Resume()
}

// StopThreeHoleTraversal 停止三孔移位测试
func (a *App) StopThreeHoleTraversal() {
	a.threeHoleService.Stop()
}

// GetThreeHoleTraversalStatus 获取三孔移位测试状态
func (a *App) GetThreeHoleTraversalStatus() types.ThreeHoleTraversalTaskStatus {
	return a.threeHoleService.GetStatus()
}

// SelectThreeHoleCalibFiles 选择三孔校准文件（弹出文件对话框）
func (a *App) SelectThreeHoleCalibFiles() []string {
	filePaths, err := wailsRuntime.OpenMultipleFilesDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择三孔校准文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "校准数据文件", Pattern: "*.dat;*.txt;*.prb"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		log.Printf("select calib files failed: %v", err)
		return []string{}
	}
	return filePaths
}

// ==================== 三孔移位测试事件发布器 ====================

// threeHoleEventPublisher 三孔移位测试事件发布器
type threeHoleEventPublisher struct {
	app *App
}

func (p *threeHoleEventPublisher) EmitProgress(event types.ThreeHoleTraversalProgressEvent) {
	wailsRuntime.EventsEmit(p.app.ctx, "three-hole:progress", event)
}

func (p *threeHoleEventPublisher) EmitRealtime(event types.ThreeHoleTraversalRealtimeEvent) {
	wailsRuntime.EventsEmit(p.app.ctx, "three-hole:realtime", event)
}

func (p *threeHoleEventPublisher) EmitComplete(event types.ThreeHoleTraversalCompleteEvent) {
	wailsRuntime.EventsEmit(p.app.ctx, "three-hole:complete", event)
}

func (p *threeHoleEventPublisher) EmitError(event types.ThreeHoleTraversalErrorEvent) {
	wailsRuntime.EventsEmit(p.app.ctx, "three-hole:error", event)
}
