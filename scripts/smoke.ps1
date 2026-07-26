# Smoke test without live Zen dependency.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$env:ADMIN_PASSWORD = "smoke-admin-password"
$env:ADMIN_SECRET = ("s" * 32)
$env:DATA_DIR = Join-Path $env:TEMP ("oc2-smoke-" + [guid]::NewGuid().ToString("N"))
$env:LISTEN = "127.0.0.1:0"
New-Item -ItemType Directory -Path $env:DATA_DIR | Out-Null

# Build binary
go build -o bin/smoke-server.exe ./cmd/server
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

# Use fixed port for simple curl
$env:LISTEN = "127.0.0.1:18646"
$proc = Start-Process -FilePath ".\bin\smoke-server.exe" -PassThru -WindowStyle Hidden
try {
  $ok = $false
  for ($i = 0; $i -lt 50; $i++) {
    try {
      $health = Invoke-RestMethod -Uri "http://127.0.0.1:18646/health" -TimeoutSec 1
      if ($health.status -eq "ok") { $ok = $true; break }
    } catch { Start-Sleep -Milliseconds 100 }
  }
  if (-not $ok) { throw "health not ready" }

  $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
  $login = Invoke-WebRequest -Uri "http://127.0.0.1:18646/api/admin/login" -Method POST `
    -ContentType "application/json" -Body '{"password":"smoke-admin-password"}' -WebSession $session
  if ($login.StatusCode -ne 200) { throw "login failed" }

  $bad = $null
  try {
    Invoke-WebRequest -Uri "http://127.0.0.1:18646/api/admin/login" -Method POST `
      -ContentType "application/json" -Body '{"password":"wrong"}' -ErrorAction Stop | Out-Null
  } catch {
    $bad = $_.Exception.Response.StatusCode.value__
  }
  if ($bad -ne 401) { throw "expected 401 for bad password, got $bad" }

  $create = Invoke-RestMethod -Uri "http://127.0.0.1:18646/api/admin/local-keys" -Method POST `
    -ContentType "application/json" -Body '{"label":"smoke"}' -WebSession $session
  $secret = $create.Secret
  if (-not $secret) { $secret = $create.secret }
  if (-not $secret) { throw "missing created secret" }

  $models = Invoke-RestMethod -Uri "http://127.0.0.1:18646/v1/models"
  if ($models.object -ne "list") { throw "models shape invalid" }

  # secret audit: ensure cookie not present in metrics/logs payload text
  $metrics = Invoke-RestMethod -Uri "http://127.0.0.1:18646/metrics"
  $metricsText = $metrics | ConvertTo-Json -Compress
  if ($metricsText -match "smoke-admin-password" -or $metricsText -match [regex]::Escape($secret)) {
    throw "secret leaked into metrics"
  }

  Write-Host "SMOKE_OK"
  exit 0
}
finally {
  if ($proc -and -not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
}
