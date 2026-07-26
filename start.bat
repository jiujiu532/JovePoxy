@echo off
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"

REM ASCII-only batch for cmd.exe (no Chinese comments)
REM Usage:
REM   start.bat           full: stop, rebuild web+embed, go build, run
REM   start.bat quick     skip web rebuild; rebuild go binary only, run
REM   start.bat run       skip all rebuilds; run existing bin\jovepoxy.exe

set "MODE=%~1"
if "%MODE%"=="" set "MODE=full"
set "PORT=6446"
set "LISTEN=127.0.0.1:%PORT%"
set "ADMIN_PASSWORD=admin123456"
set "ADMIN_SECRET=0123456789abcdef0123456789abcdef"
set "DATA_DIR=%~dp0data"
set "COOKIE_SECURE=false"
set "BIN=%~dp0bin\jovepoxy.exe"

echo.
echo ========================================
echo  JovePoxy start  mode=%MODE%
echo  URL:      http://%LISTEN%/
echo  password: %ADMIN_PASSWORD%
echo ========================================
echo.

REM --- stop previous listener on PORT ---
call "%~dp0stop.bat" %PORT%
if errorlevel 1 (
  echo [warn] stop.bat returned %ERRORLEVEL%, continue anyway
)

if /I "%MODE%"=="run" goto :ensure_bin_only
if /I "%MODE%"=="quick" goto :go_build
if /I not "%MODE%"=="full" (
  echo [error] unknown mode "%MODE%". Use: full ^| quick ^| run
  exit /b 2
)

REM --- full: frontend build + embed ---
where pnpm >nul 2>&1
if errorlevel 1 (
  echo [error] pnpm not found in PATH. Install pnpm 10.x first.
  exit /b 1
)

if not exist "web\package.json" (
  echo [error] web\package.json missing
  exit /b 1
)

echo [info] web: pnpm install --frozen-lockfile
pushd web
call pnpm install --frozen-lockfile
if errorlevel 1 (
  echo [error] pnpm install failed
  popd
  exit /b 1
)

echo [info] web: pnpm build
call pnpm build
if errorlevel 1 (
  echo [error] pnpm build failed
  popd
  exit /b 1
)
popd

if not exist "web\dist\index.html" (
  echo [error] web\dist\index.html missing after build
  exit /b 1
)

echo [info] embed web\dist -^> internal\webui\dist
if not exist "internal\webui\dist" mkdir "internal\webui\dist"
powershell -NoProfile -Command ^
  "New-Item -ItemType Directory -Force -Path 'internal/webui/dist' | Out-Null; " ^
  "Get-ChildItem -Path 'internal/webui/dist' -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue; " ^
  "Copy-Item -Path 'web/dist/*' -Destination 'internal/webui/dist/' -Recurse -Force"
if errorlevel 1 (
  echo [error] embed copy failed
  exit /b 1
)

:go_build
where go >nul 2>&1
if errorlevel 1 (
  echo [error] go not found in PATH
  exit /b 1
)

if not exist "bin" mkdir "bin"
echo [info] go build -o bin\jovepoxy.exe .\cmd\server
go build -o "bin\jovepoxy.exe" .\cmd\server
if errorlevel 1 (
  echo [error] go build failed
  exit /b 1
)
goto :run

:ensure_bin_only
if not exist "%BIN%" (
  echo [error] %BIN% not found. Run: start.bat   or   start.bat quick
  exit /b 1
)

:run
if not exist "%DATA_DIR%" mkdir "%DATA_DIR%"

echo.
echo [ok] starting %BIN%
echo      listen  %LISTEN%
echo      data    %DATA_DIR%
echo      Press Ctrl+C to stop
echo.

"%BIN%"
set "RC=%ERRORLEVEL%"
echo.
echo [exit] code=%RC%
exit /b %RC%
