#!/bin/bash
# Deploy GoOnDVR from GitHub to AWS Free Tier EC2
# Run this directly on EC2 instance

set -e  # Exit on error

echo "🚀 Deploying GoOnDVR from GitHub..."
echo "===================================="
echo ""

REPO_URL="https://github.com/vasud3v/record.git"
APP_DIR="/home/ubuntu/goondvr"
BRANCH="main"

# Step 1: Install Git if not present
echo "📦 Step 1: Installing Git..."
if ! command -v git &> /dev/null; then
    sudo apt update -qq
    sudo apt install -y git
    echo "✅ Git installed!"
else
    echo "✅ Git already installed!"
fi

# Step 2: Clone or update repository
echo ""
echo "📥 Step 2: Cloning repository..."
if [ -d "$APP_DIR" ]; then
    echo "⚠️  Directory exists, pulling latest changes..."
    cd "$APP_DIR"
    git fetch origin
    git reset --hard origin/$BRANCH
    git pull origin $BRANCH
else
    echo "📦 Cloning fresh repository..."
    git clone "$REPO_URL" "$APP_DIR"
    cd "$APP_DIR"
fi
echo "✅ Repository ready!"

# Step 3: Install Docker
echo ""
echo "🐳 Step 3: Installing Docker..."
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    sudo usermod -aG docker ubuntu
    rm get-docker.sh
    echo "✅ Docker installed!"
else
    echo "✅ Docker already installed!"
fi

# Step 4: Install Docker Compose
echo ""
echo "🐳 Step 4: Installing Docker Compose..."
if ! docker compose version &> /dev/null; then
    sudo apt update -qq
    sudo apt install -y docker-compose-plugin
    echo "✅ Docker Compose installed!"
else
    echo "✅ Docker Compose already installed!"
fi

# Step 5: Create necessary directories
echo ""
echo "📁 Step 5: Creating directories..."
mkdir -p "$APP_DIR/videos/completed"
mkdir -p "$APP_DIR/database"
mkdir -p "$APP_DIR/conf"
echo "✅ Directories created!"

# Step 6: Setup environment file
echo ""
echo "⚙️  Step 6: Setting up environment..."
if [ ! -f "$APP_DIR/.env" ]; then
    if [ -f "$APP_DIR/.env.example" ]; then
        cp "$APP_DIR/.env.example" "$APP_DIR/.env"
        echo "✅ .env file created from example"
    else
        echo "⚠️  No .env.example found, skipping"
    fi
else
    echo "✅ .env file already exists"
fi

# Step 7: Build and start containers
echo ""
echo "🏗️  Step 7: Building and starting containers..."
echo "⏳ This will take 5-10 minutes on first run..."
cd "$APP_DIR"
sudo docker compose down 2>/dev/null || true
sudo docker compose up -d --build

# Step 8: Wait for containers to be healthy
echo ""
echo "⏳ Step 8: Waiting for containers to be healthy..."
sleep 30

# Step 9: Check container status
echo ""
echo "📊 Step 9: Container Status:"
sudo docker compose ps

# Step 10: Setup automation
echo ""
echo "🤖 Step 10: Setting up automation..."
chmod +x "$APP_DIR/scripts/"*.sh
"$APP_DIR/scripts/setup-full-automation.sh"

# Step 11: Display system info
echo ""
echo "✅ DEPLOYMENT COMPLETE!"
echo "======================"
echo ""
echo "📊 System Information:"
echo ""
echo "Memory:"
free -h | grep Mem
echo ""
echo "Disk:"
df -h / | tail -1
echo ""
echo "Containers:"
cd "$APP_DIR"
sudo docker compose ps --format 'table {{.Name}}\t{{.Status}}'
echo ""

# Get public IP
PUBLIC_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || echo "UNKNOWN")

echo "🌐 Web UI: http://${PUBLIC_IP}:8080"
echo ""
echo "🎯 Capacity (c7i-flex.large):"
echo "  • RAM: 4 GB"
echo "  • Byparr instances: 4 (handles ~40 channels)"
echo "  • Auto-scaling: Enabled (1-6 instances)"
echo "  • Auto-cleanup: Enabled (every 2 hours)"
echo ""
echo "📚 Next Steps:"
echo "  1. Open web UI: http://${PUBLIC_IP}:8080"
echo "  2. Add 20-30 channels to start"
echo "  3. Monitor: sudo docker compose logs -f recorder"
echo ""
echo "📝 Logs:"
echo "  • Auto-scaler: tail -f /home/ubuntu/goondvr/auto-scaler.log"
echo "  • Cleanup: tail -f /home/ubuntu/goondvr/upload-cleanup.log"
echo "  • Recorder: cd /home/ubuntu/goondvr && sudo docker compose logs recorder"
echo ""
echo "🎉 Your free tier recorder is ready!"
