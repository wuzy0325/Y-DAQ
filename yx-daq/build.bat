@echo off
REM YX-DAQ 构建脚本 (Wails v3)
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
echo   YX-DAQ 构建 (Wails v3)
echo ============================================

echo [1/1] Wails v3 构建...
wails3 task build
if errorlevel 1 (
    echo 构建失败!
    goto :error
)

echo 构建完成!
echo 产物: bin\yx-daq.exe
goto :end

:build_nsis
echo ============================================
echo   YX-DAQ 构建 + NSIS安装包 (Wails v3)
echo ============================================

echo [1/1] Wails v3 构建 + 打包...
wails3 task package
if errorlevel 1 (
    echo 打包失败!
    echo 请确保已安装NSIS (https://nsis.sourceforge.io/Download)
    goto :error
)

echo 安装包构建完成!
echo 产物: bin\yx-daq-amd64-installer.exe
goto :end

:error
echo 构建过程中出现错误!
exit /b 1

:end
endlocal
