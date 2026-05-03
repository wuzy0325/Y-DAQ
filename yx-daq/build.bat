@echo off
REM YX-DAQ 构建脚本
REM 用法:
REM   build.bat          - 仅构建exe
REM   build.bat nsis     - 构建exe + NSIS安装包
REM   build.bat clean    - 清理构建产物

setlocal

set PROJECT_DIR=%~dp0
cd /d "%PROJECT_DIR%"

if "%1"=="clean" goto :clean
if "%1"=="nsis" goto :build_nsis
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
echo   YX-DAQ 构建
echo ============================================

echo [1/3] 检查Go编译...
go build ./...
if errorlevel 1 (
    echo Go编译失败!
    goto :error
)

echo [2/3] Wails构建...
wails build
if errorlevel 1 (
    echo Wails构建失败!
    goto :error
)

echo [3/3] 构建完成!
echo 产物: build\bin\yx-daq.exe
goto :end

:build_nsis
echo ============================================
echo   YX-DAQ 构建 + NSIS安装包
echo ============================================

echo [1/4] 检查Go编译...
go build ./...
if errorlevel 1 (
    echo Go编译失败!
    goto :error
)

echo [2/4] Wails构建 (NSIS)...
wails build -platform windows/amd64 -nsis
if errorlevel 1 (
    echo Wails NSIS构建失败!
    echo 请确保已安装NSIS (https://nsis.sourceforge.io/Download)
    goto :error
)

echo [3/4] 检查安装包...
if exist "build\bin\yx-daq-amd64-installer.exe" (
    echo [4/4] 安装包构建完成!
    echo 产物: build\bin\yx-daq-amd64-installer.exe
) else (
    echo [4/4] 安装包未找到，请检查NSIS是否正确安装
)

goto :end

:error
echo 构建过程中出现错误!
exit /b 1

:end
endlocal
