# YX-DAQ 开发进度文档

> 最后更新: 2026-04-12 (第三轮迭代完成)

## 项目概述

基于 Wails v3 (Go + Vue 3) 的五孔探针校准系统桌面端 APP，目标平台 Windows。

## 技术栈

| 层级 | 技术 | 版本 |
|------|------|------|
| 桌面框架 | Wails | v2.12.0 |
| 后端 | Go | 1.23.0 |
| 前端 | Vue 3 + TypeScript | 3.x |
| UI | Element Plus + SCSS | - |
| 状态管理 | Pinia | 3.x |
| 图表 | ECharts | 6.x |
| PDF生成 | go-pdf/fpdf | v0.9.0 |
| 前端测试 | Vitest + @vue/test-utils | 4.x |
| 构建 | Wails CLI + Vite + NSIS | - |

## 开发进度

### 阶段 1: 项目脚手架与基础框架 ✅

- [x] Wails 项目初始化 (`wails init -n yx-daq -t vue-ts`)
- [x] 窗口参数配置 (1440×900, 最小 1280×720)
- [x] 前端依赖安装 (vue-router, pinia, element-plus, echarts, sass)
- [x] 样式系统迁移 (variables.scss, global.scss, theme-variables.scss)
- [x] 布局框架 (MainLayout: 侧边栏 + 顶栏 + 内容区)
- [x] 路由配置 (5个页面: 仪表盘/设备/运动/校准/数据)
- [x] Go 后端目录结构 (internal/types, driver, manager, calibration, storage, scanner)

### 阶段 2: Go 后端 - 共享类型与配置持久化 ✅

- [x] `internal/types/device.go` - 设备类型
- [x] `internal/types/motion.go` - 运动类型
- [x] `internal/types/calibration.go` - 校准类型 + 事件类型
- [x] `internal/types/constants.go` - 全局常量
- [x] `internal/storage/config_store.go` - JSON 配置持久化 (原子写入, 损坏修复)

### 阶段 3: Go 后端 - XY-DAQ16 设备驱动 ✅

- [x] `internal/driver/xy_daq16.go` - TCP 连接, 2字节长度前缀拆包, 帧解析, 采集控制, 指数退避重连
- [x] `internal/driver/simulated_device.go` - 模拟设备
- [x] `internal/scanner/daq_scanner.go` - UDP 设备扫描

### 阶段 4: Go 后端 - B140 运动控制器驱动 ✅

- [x] `internal/driver/b140.go` - B140Driver + B140MotionController (完整运动控制接口)
- [x] `internal/driver/simulated_motion.go` - 模拟运动控制器

### 阶段 5: Go 后端 - 管理器层与数据中枢 ✅

- [x] `internal/manager/device_manager.go` - DeviceManager
- [x] `internal/manager/motion_manager.go` - MotionControllerManager (10Hz轮询)
- [x] `internal/manager/acquisition_hub.go` - AcquisitionHub (20Hz发布)

### 阶段 6: Go 后端 - 五孔探针校准模块 ✅

- [x] `internal/calibration/formulas.go` - 五孔系数计算 (Kα, Kβ, CPT, CPS), 平均值, 标准差, 通道角色匹配
- [x] `internal/calibration/service.go` - CalibrationService (校准主循环, 暂停/恢复/停止, 事件发布)
- [x] `internal/calibration/encoder_compensation.go` - 编码器补偿状态机 (waiting→settling→checking→compensating→succeeded/failed)
- [x] `internal/calibration/sphere_tank_gate.go` - 球罐稳定门控 (压力变化率监测, 稳定判定)

### 阶段 7: Go 后端 - Wails API 层 ✅

- [x] `app.go` - 30+ 个 Wails 绑定方法 + 5 个事件推送

### 阶段 8: 前端 - 基础组件与样式系统 ✅

- [x] 深色霓虹科技风样式系统
- [x] `components/GlassCard.vue` - 玻璃态卡片组件
- [x] `components/ChartPanel.vue` - ECharts 封装组件 (自动resize, 深色主题)
- [x] `components/StatusIndicator.vue` - 状态指示器 (connected/running/error/warning)
- [x] `components/ValueDisplay.vue` - 数值显示组件 (精度/单位/颜色)
- [x] MainLayout 布局

### 阶段 9: 前端 - Pinia Stores ✅

- [x] `stores/device.ts`, `stores/motion.ts`, `stores/calibration.ts`

### 阶段 10: 前端 - 仪表盘页面 ✅

- [x] ECharts 实时压力折线图 (霓虹配色, 自动滚动)
- [x] 设备/运动/校准状态卡片
- [x] 实时通道数值面板

### 阶段 11: 前端 - 设备管理页面 ✅

- [x] 设备列表表格 + 连接/采集控制
- [x] ECharts 实时通道折线图
- [x] 设备配置表单 (添加设备对话框, 支持模拟/XY-DAQ16)
- [x] UDP 扫描按钮

### 阶段 12: 前端 - 运动控制页面 ✅

- [x] 4轴面板 (位置/点动/定位/回零)
- [x] 急停按钮 (霓虹红发光)
- [x] 运动控制器配置表单 (添加控制器对话框, 支持模拟/B140)
- [x] 轴颜色区分 (X=紫, Y=青, Z=绿, U=橙)

### 阶段 13: 前端 - 五孔校准页面 ✅

- [x] 校准配置表单 (类型/轴/范围/步数/驻留/采样)
- [x] 实时数据面板 (P1-P5 + Kα/Kβ/CPT/CPS, 霓虹配色)
- [x] 校准进度条
- [x] 校准结果表格 (完整列: α,β,P1-P5,Kα,Kβ,CPT,CPS,采样数,标准差)
- [x] 系数等值线图 (α-β平面散点+visualMap, Kα/Kβ/CPT/CPS切换)
- [x] CSV 导出 (UTF-8 BOM, 前端Blob下载)

### 阶段 14: 前端 - 数据管理页面 ✅

- [x] 录制控制, 数据信息

### 阶段 15: Go 后端 - 数据存储服务 ✅

- [x] `internal/storage/data_storage.go` - DataStorageService (录制管理, CSV导出, UTF-8 BOM)

### 阶段 16: Go 单元测试 ✅

- [x] `internal/calibration/formulas_test.go` - 8个测试用例 (系数计算/平均值/标准差/除零保护/通道匹配)
- [x] `internal/storage/config_store_test.go` - 4个测试用例 (保存加载/设置/批量加载/损坏修复)
- [x] 全部通过 ✅

### 阶段 17: 构建验证 ✅

- [x] Go 编译通过 (`go build ./...`)
- [x] Wails v3 构建成功 (`wails3 task build` → `bin/yx-daq.exe`)

### 阶段 18: PDF 报告导出 ✅

- [x] `internal/storage/pdf_report.go` - PdfReportService (go-pdf/fpdf)
- [x] PDF报告内容: 标题/报告信息/探针通道配置/校准结果表格/统计摘要
- [x] `app.go` 新增 `ExportCalibrationPDF()` API (Wails SaveFileDialog + PDF生成)
- [x] 前端 CalibrationView 新增"导出PDF"按钮

### 阶段 19: 数据回放功能 ✅

- [x] `app.go` 新增 `LoadCSVFile()` API (Wails OpenFileDialog + 文件读取)
- [x] `app.go` 新增 `ListRecordingFiles()` / `GetDataDir()` API
- [x] DataView 重构: 录制控制 + 录制文件列表 + 数据回放面板
- [x] CSV解析 (BOM处理, 通道分组)
- [x] ECharts回放图表 (按通道分组折线, 霓虹配色)
- [x] 回放控制: 播放/暂停/重置/速度调节 (0.25x~4x)

### 阶段 20: 校准点可视化编辑 ✅

- [x] `components/CalibPointEditor.vue` - Canvas α-β平面校准点编辑器
- [x] 功能: 拖拽移动校准点 / 添加点 / 删除选中点 / 重置网格
- [x] 交互: 鼠标点击选中 / 拖拽移动 / 范围限制 / 坐标实时显示
- [x] 视觉: 霓虹发光选中效果 / 序号标注 / 坐标轴刻度 / HiDPI支持
- [x] CalibrationView 集成: v-model双向绑定, 启动校准使用编辑器中的点

### 阶段 21: NSIS 安装包打包 ✅

- [x] `wails.json` 新增 info 配置 (companyName/productName/productVersion/copyright)
- [x] `build.bat` 一键构建脚本 (build / build nsis / clean)
- [x] Wails v3 + NSIS 集成: `build.bat nsis` / `wails3 task package`

### 阶段 22: 前端组件测试 ✅

- [x] Vitest + @vue/test-utils + happy-dom 配置
- [x] `components/__tests__/ValueDisplay.test.ts` - 6个测试 (数值渲染/精度/单位/NaN/颜色)
- [x] `components/__tests__/StatusIndicator.test.ts` - 7个测试 (状态类/标签/动画/全部状态类型)
- [x] `components/__tests__/GlassCard.test.ts` - 7个测试 (插槽/标题/图标/操作/提升)
- [x] 全部 20 个测试通过 ✅

## 构建产物

```
yx-daq/bin/yx-daq.exe                    (Windows 可执行文件, 1440×900 窗口)
yx-daq/bin/yx-daq-amd64-installer.exe    (NSIS 安装包, 需 NSIS/makensis)
```

> 目录结构详见 `architecture.md`，此处不再重复。

## 待完善项

(全部完成 ✅)
