param(
  [Parameter(Mandatory = $true)]
  [string]$ApiImage,

  [Parameter(Mandatory = $true)]
  [string]$FrontendImage,

  [switch]$FollowLogs
)

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Split-Path -Parent $scriptDir
Set-Location $rootDir

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw 'docker not found'
}

$overrideFile = '.env.images'
@"
API_IMAGE=$ApiImage
FRONTEND_IMAGE=$FrontendImage
"@ | Set-Content -Path $overrideFile -Encoding UTF8

Write-Host '========================================' -ForegroundColor Cyan
Write-Host ' ClawLink Production Rollback ' -ForegroundColor Cyan
Write-Host '========================================' -ForegroundColor Cyan
Write-Host "API_IMAGE=$ApiImage"
Write-Host "FRONTEND_IMAGE=$FrontendImage"
Write-Host ''

docker compose --env-file $overrideFile up -d --force-recreate

Write-Host ''
Write-Host 'Container status:' -ForegroundColor Green
docker compose --env-file $overrideFile ps

if ($FollowLogs) {
  Write-Host ''
  docker compose --env-file $overrideFile logs -f --tail=100
} else {
  Write-Host ''
  docker compose --env-file $overrideFile logs --tail=40
}
