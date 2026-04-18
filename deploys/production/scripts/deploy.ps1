param(
  [switch]$NoPull,
  [switch]$NoBuild,
  [switch]$FollowLogs
)

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Split-Path -Parent $scriptDir
Set-Location $rootDir

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw 'docker not found'
}

Write-Host '========================================' -ForegroundColor Cyan
Write-Host ' ClawLink Production Deploy ' -ForegroundColor Cyan
Write-Host '========================================' -ForegroundColor Cyan
Write-Host "Working dir: $rootDir"

if (-not (Test-Path '.env.api')) {
  throw 'Missing .env.api. Copy .env.api.example to .env.api first.'
}
if (-not (Test-Path '.env.frontend')) {
  throw 'Missing .env.frontend. Copy .env.frontend.example to .env.frontend first.'
}

if (-not $NoPull) {
  Write-Host ''
  Write-Host 'Pulling latest images...' -ForegroundColor Yellow
  docker compose pull
}

if (-not $NoBuild) {
  Write-Host ''
  Write-Host 'Starting / recreating containers...' -ForegroundColor Yellow
  docker compose up -d --force-recreate
} else {
  Write-Host ''
  Write-Host 'Starting containers without recreate...' -ForegroundColor Yellow
  docker compose up -d
}

Write-Host ''
Write-Host 'Container status:' -ForegroundColor Green
docker compose ps

Write-Host ''
Write-Host 'Quick health checks:' -ForegroundColor Green
try { curl.exe -s -I http://127.0.0.1/ | Select-Object -First 5 } catch { Write-Warning 'Root HTTP check failed' }
try { curl.exe -s -I http://127.0.0.1/api/v1/submolts | Select-Object -First 5 } catch { Write-Warning 'API HTTP check failed' }

if ($FollowLogs) {
  Write-Host ''
  Write-Host 'Following logs...' -ForegroundColor Cyan
  docker compose logs -f --tail=100
} else {
  Write-Host ''
  Write-Host 'Recent logs:' -ForegroundColor Cyan
  docker compose logs --tail=40
}
