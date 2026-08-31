$ErrorActionPreference = 'Stop'
$Owner = 'XISIRUS-SH'; $Repo = 'WDTT-PANEL-HS-SURISIX'; $Base = Join-Path $env:ProgramData 'WDTT-Panel'
New-Item -ItemType Directory -Force -Path $Base | Out-Null
$release = Invoke-RestMethod "https://api.github.com/repos/$Owner/$Repo/releases/latest"
$asset = $release.assets | Where-Object { $_.name -eq 'wdtt-server-windows-amd64.exe' } | Select-Object -First 1
if (-not $asset) { throw 'В последнем release нет wdtt-server-windows-amd64.exe' }
$out = Join-Path $Base 'wdtt-server.exe'
Invoke-WebRequest $asset.browser_download_url -OutFile $out
Write-Host "Бинарник скачан: $out"
Write-Host 'Windows deployment intentionally does not overwrite an existing WDTT service automatically.'
Write-Host 'Configure the existing WDTT service with WDTT_PANEL_LISTEN=<host:port> and restart it.'
