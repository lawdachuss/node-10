# Auto-scale Byparr instances based on channel count
# Usage: .\scripts\scale-byparr.ps1

$CHANNELS_FILE = "conf/channels.json"

# Read and parse channels.json
$channels = Get-Content $CHANNELS_FILE | ConvertFrom-Json

# Count active channels (not paused)
$activeChannels = ($channels | Where-Object { $_.is_paused -eq $false }).Count

# Calculate needed Byparr instances (1 instance per 10 channels, minimum 3, maximum 20)
$neededInstances = [Math]::Ceiling($activeChannels / 10)
if ($neededInstances -lt 3) { $neededInstances = 3 }
if ($neededInstances -gt 20) { $neededInstances = 20 }

Write-Host "📊 Active channels: $activeChannels" -ForegroundColor Cyan
Write-Host "🔧 Needed Byparr instances: $neededInstances" -ForegroundColor Yellow

# Scale the byparr service
docker compose up -d --scale byparr=$neededInstances --no-recreate

Write-Host "✅ Scaled to $neededInstances Byparr instances" -ForegroundColor Green
