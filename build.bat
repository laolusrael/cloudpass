@echo off
setlocal enabledelayedexpansion

set VERSION=0.1.0
if not "%VERSION%"=="" set VERSION=%VERSION%

echo === Building CloudPass v%VERSION% ===

cd /d "%~dp0"

echo >>> Cleaning up...
if exist "ui\build" rmdir /S /Q "ui\build"
if exist "api\internal\web\build" rmdir /S /Q "api\internal\web\build"

echo >>> Building UI...
cd ui
call npm ci --silent
call npm run build
cd ..

echo >>> Copying UI to embed directory...
if not exist "api\internal\web\build" mkdir "api\internal\web\build"
xcopy /E /Q /Y ui\build\* api\internal\web\build\

echo >>> Building Go API...
cd api

echo >>> Building for Windows amd64...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -o "..\cloudpass-windows-amd64.exe" .\cmd\server

cd ..

echo >>> Setup local binary...
copy /Y cloudpass-windows-amd64.exe cloudpass.exe

echo === Build Complete ===
echo Binaries created:
dir cloudpass-*

endlocal
