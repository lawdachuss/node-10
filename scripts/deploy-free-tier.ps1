# Deploy GoOnDVR to AWS Free Tier EC2
# Run this from your Windows machine

param(
    [Parameter(Mandatory=$true)]
    [string]$EC2_IP,
    
    [Parameter(Mandatory=$false)]
    [string]$KeyPath = "aws-secrets/aws-key.pem"
)

Write-Host "🆓 Deploying GoOnDVR to AWS Free Tier..." -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if key exists
if (-not (Test-Path $KeyPath)) {
    Write-Host "❌ SSH key not found: $KeyPath" -ForegroundColor Red
    exit 1
}

Write-Host "📦 Step 1: Creating remote directory..." -ForegroundColor Yellow
ssh -i $KeyPath ubuntu@${EC2_IP} "mkdir -p /home/ubuntu/goondvr"

Write-Host "📤 Step 2: Uploading project files..." -ForegroundColor Yellow
Write-Host "   This may take a few minutes..." -ForegroundColor Gray

# Upload all necessary files
scp -i $KeyPath -r `
    docker-compose.yml `
    Dockerfile `
    go.mod `
    go.sum `
    main.go `
    nginx.conf `
    .env.example `
    ubuntu@${EC2_IP}:/home/ubuntu/goondvr/

# Upload directories
scp -i $KeyPath -r channel ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r chaturbate ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r config ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r entity ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r internal ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r manager ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r notifier ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r router ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r server ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r site ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r stripchat ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r supabase ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r uploader ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r conf ubuntu@${EC2_IP}:/home/ubuntu/goondvr/
scp -i $KeyPath -r scripts ubuntu@${EC2_IP}:/home/ubuntu/goondvr/

Write-Host "✅ Files uploaded!" -ForegroundColor Green
Write-Host ""

Write-Host "🐳 Step 3: Installing Docker..." -ForegroundColor Yellow
ssh -i $KeyPath ubuntu@${EC2_IP} @"
    # Update system
    sudo apt update -qq
    
    # Install Docker if not already installed
    if ! command -v docker &> /dev/null; then
        curl -fsSL https://get.docker.com -o get-docker.sh
        sudo sh get-docker.sh
        sudo usermod -aG docker ubuntu
        rm get-docker.sh
    fi
    
    # Install Docker Compose plugin
    if ! docker compose version &> /dev/null; then
        sudo apt install -y docker-compose-plugin
    fi
    
    echo '✅ Docker installed!'
"@

Write-Host "🏗️  Step 4: Building and starting containers..." -ForegroundColor Yellow
Write-Host "   This will take 5-10 minutes on first run..." -ForegroundColor Gray
ssh -i $KeyPath ubuntu@${EC2_IP} @"
    cd /home/ubuntu/goondvr
    
    # Create necessary directories
    mkdir -p videos/completed database
    
    # Copy example env if needed
    if [ ! -f .env ]; then
        cp .env.example .env 2>/dev/null || true
    fi
    
    # Build and start (FREE TIER: 1 Byparr instance)
    sudo docker compose up -d --build
    
    echo ''
    echo '⏳ Waiting for containers to be healthy...'
    sleep 30
    
    echo ''
    echo '📊 Container Status:'
    sudo docker compose ps
"@

Write-Host ""
Write-Host "🔧 Step 5: Setting up automation..." -ForegroundColor Yellow
ssh -i $KeyPath ubuntu@${EC2_IP} @"
    cd /home/ubuntu/goondvr
    
    # Make scripts executable
    chmod +x scripts/*.sh
    
    # Setup full automation
    ./scripts/setup-full-automation.sh
"@

Write-Host ""
Write-Host "✅ DEPLOYMENT COMPLETE!" -ForegroundColor Green
Write-Host "======================" -ForegroundColor Green
Write-Host ""
Write-Host "🌐 Web UI: http://${EC2_IP}:8080" -ForegroundColor Cyan
Write-Host ""
Write-Host "📊 System Info:" -ForegroundColor Yellow
ssh -i $KeyPath ubuntu@${EC2_IP} @"
    echo '  Memory:'
    free -h | grep Mem
    echo ''
    echo '  Disk:'
    df -h / | tail -1
    echo ''
    echo '  Containers:'
    sudo docker compose ps --format 'table {{.Name}}\t{{.Status}}'
"@

Write-Host ""
Write-Host "🎯 Free Tier Limits:" -ForegroundColor Yellow
Write-Host "  • Max channels: ~10 (with 1 Byparr instance)" -ForegroundColor Gray
Write-Host "  • Disk space: 30 GB (auto-cleanup enabled)" -ForegroundColor Gray
Write-Host "  • Memory: 1 GB (optimized)" -ForegroundColor Gray
Write-Host ""
Write-Host "📚 Next Steps:" -ForegroundColor Yellow
Write-Host "  1. Add channels via web UI: http://${EC2_IP}:8080" -ForegroundColor Gray
Write-Host "  2. Monitor logs: ssh -i $KeyPath ubuntu@${EC2_IP} 'cd /home/ubuntu/goondvr && sudo docker compose logs -f'" -ForegroundColor Gray
Write-Host "  3. Check memory: ssh -i $KeyPath ubuntu@${EC2_IP} 'free -h'" -ForegroundColor Gray
Write-Host ""
Write-Host "⚠️  Important:" -ForegroundColor Red
Write-Host "  • Start with 5-8 channels to test stability" -ForegroundColor Gray
Write-Host "  • Monitor memory usage (should stay < 85%)" -ForegroundColor Gray
Write-Host "  • Auto-cleanup runs every 2 hours" -ForegroundColor Gray
Write-Host ""
Write-Host "🎉 Your free tier recorder is ready!" -ForegroundColor Green
