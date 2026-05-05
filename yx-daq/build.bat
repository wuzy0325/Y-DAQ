@echo off
setlocal enabledelayedexpansion
REM YX-DAQ 构建脚本 (Wails v3)
REM 用法:
REM   build.bat          - 仅构建exe
REM   build.bat clean    - 清理构建产物
REM 注意: Wails v3 alpha 未支持 NSIS 安装包，仅构建 exe

set PROJECT_DIR=%~dp0
cd /d "%PROJECT_DIR%"

if "%1"=="clean" goto :clean
goto :build

:clean
echo [1/2] 清理构建产物...
if exist "build\bin" rmdir /s /q "build\bin"
echo [2/2] 清理前端产物...
if exist "frontend\dist" rmdir /s /q "frontend\dist"
echo 清理完成.
goto :end

:build
echo ============================================
echo   YX-DAQ 构建 (Wails v3)
echo ============================================

echo [1/4] 检查Go编译...
set GOPROXY=https://goproxy.cn,direct
set GOTOOLCHAIN=go1.25.9
go build ./...
if errorlevel 1 (
    echo Go编译失败!
    goto :error
)

echo [2/4] 安装前端依赖...
cd frontend
call npm install --silent
if errorlevel 1 (
    echo npm install 失败!
    goto :error
)

echo [3/4] 构建前端...
call npm run build
if errorlevel 1 (
    echo 前端构建失败!
    goto :error
)
cd ..

echo [4/4] 构建exe...
if not exist "build\bin" mkdir "build\bin"
go build -o build\bin\yx-daq.exe .
if errorlevel 1 (
    echo Go二进制构建失败!
    goto :error
)

echo 构建完成!
echo 产物: build\bin\yx-daq.exe
goto :end

:error
echo 构建过程中出现错误!
exit /b 1

:end
endlocal
