@echo off
REM YX-DAQ 开发脚本
REM 用法:
REM   dev.bat          - 编译并启动开发模式 (wails dev)
REM   dev.bat build    - 编译并启动生产构建 (build\bin\yx-daq.exe)
REM   dev.bat run      - 直接运行已构建的exe (不重新编译)

setlocal

set PROJECT_DIR=%~dp0
cd /d "%PROJECT_DIR%"

if "%1"=="build" goto :build_and_run
if "%1"=="run" goto :run_only
goto :dev

:dev
echo ============================================
echo   YX-DAQ 开发模式 (热重载)
echo ============================================

echo [1/2] 安装前端依赖...
cd frontend
call npm install
if errorlevel 1 (
    echo 前端依赖安装失败!
    goto :error
)
cd ..

echo [2/2] 启动 wails dev...
wails dev
goto :end

:build_and_run
echo ============================================
echo   YX-DAQ 编译 + 运行
echo ============================================

echo [1/4] 安装前端依赖...
cd frontend
call npm install
if errorlevel 1 (
    echo 前端依赖安装失败!
    goto :error
)
cd ..

echo [2/4] 检查Go编译...
go build ./...
if errorlevel 1 (
    echo Go编译失败!
    goto :error
)

echo [3/4] Wails构建...
wails build
if errorlevel 1 (
    echo Wails构建失败!
    goto :error
)

echo [4/4] 启动应用...
if not exist "build\bin\yx-daq.exe" (
    echo 构建产物未找到!
    goto :error
)
start "" "build\bin\yx-daq.exe"
echo 应用已启动: build\bin\yx-daq.exe
goto :end

:run_only
echo ============================================
echo   YX-DAQ 运行已构建的应用
echo ============================================

if not exist "build\bin\yx-daq.exe" (
    echo 构建产物未找到，请先运行: dev.bat build
    goto :error
)
start "" "build\bin\yx-daq.exe"
echo 应用已启动: build\bin\yx-daq.exe
goto :end

:error
echo 执行过程中出现错误!
exit /b 1

:end
endlocal
