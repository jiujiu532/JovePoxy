@echo off
setlocal EnableExtensions
cd /d "%~dp0"

REM ASCII-only batch for cmd.exe
REM Usage:
REM   stop.bat          stop listener on 6446
REM   stop.bat 6446     stop listener on given port

set "PORT=%~1"
if "%PORT%"=="" set "PORT=6446"

echo [info] stop port %PORT% ...

powershell -NoProfile -Command ^
  "$p=%PORT%;" ^
  "$conns=Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue;" ^
  "if(-not $conns){ Write-Host '[info] nothing listening on' $p; exit 0 };" ^
  "$pids=$conns.OwningProcess | Select-Object -Unique;" ^
  "foreach($id in $pids){" ^
  "  try {" ^
  "    $proc=Get-Process -Id $id -ErrorAction Stop;" ^
  "    Write-Host ('[info] kill pid={0} name={1}' -f $id, $proc.ProcessName);" ^
  "    Stop-Process -Id $id -Force -ErrorAction Stop;" ^
  "  } catch {" ^
  "    Write-Host ('[warn] could not kill pid={0}: {1}' -f $id, $_.Exception.Message);" ^
  "  }" ^
  "};" ^
  "Start-Sleep -Milliseconds 300;" ^
  "$left=Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue;" ^
  "if($left){ Write-Host '[error] still listening on' $p; exit 1 } else { Write-Host '[ok] port' $p 'is free'; exit 0 }"

exit /b %ERRORLEVEL%
