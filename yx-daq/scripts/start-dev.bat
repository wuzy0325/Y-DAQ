@echo off
echo === YX-DAQ ===

echo [1/2] Stopping existing instance...
taskkill //IM yx-daq.exe //F 2>nul

echo [2/2] Launching...
cd /d "%~dp0.."
start "YX-DAQ" bin\yx-daq.exe

echo === Done ===
