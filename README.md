# YX-DAQ 五孔探针校准系统

基于 Wails v2 的桌面应用，后端 Go + 前端 Vue 3。

## 技术栈

- **后端**: Go 1.23+ / Wails v2.12.0 / go-pdf/fpdf (PDF报告导出)
- **前端**: Vue 3 + TypeScript + Vite 3 + Element Plus + ECharts 6 + Pinia 3 + Vue Router 4 + Sass

## 环境要求

- Go 1.23+
- Node.js 16+
- Wails CLI v2 (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- Windows (当前仅支持 Windows 平台)

## 开发模式

```bash
wails dev
```

启动 Vite 开发服务器，支持前端热重载。Go 方法可通过 http://localhost:34115 在浏览器中访问和调试。

## 构建

### 使用构建脚本 (推荐)

```bash
build.bat           # 仅构建 exe → build\bin\yx-daq.exe
build.bat nsis      # 构建 exe + NSIS 安装包 → build\bin\yx-daq-amd64-installer.exe
build.bat clean     # 清理构建产物
```

构建 NSIS 安装包需要先安装 [NSIS](https://nsis.sourceforge.io/Download)。

### 手动构建

```bash
wails build                              # 构建生产包
wails build --target windows/amd64 --nsis # 构建 Windows 安装包
```

## 前端单独操作

```bash
cd frontend
npm install       # 安装依赖
npm run dev       # 启动 Vite 开发服务器
npm run build     # 类型检查 + 构建 (vue-tsc --noEmit && vite build)
npm run test      # 运行测试 (vitest)
```

## 项目配置

- `wails.json` — Wails 项目配置 (名称、构建命令、产品信息等)
- `frontend/vite.config.ts` — Vite 构建配置
- `frontend/package.json` — 前端依赖和脚本
- `go.mod` — Go 模块依赖

## 应用信息

- 窗口标题: YX-DAQ 五孔探针校准系统
- 默认与最小尺寸: 1440 x 900
- 产品版本: 1.0.0
