#!/bin/bash
# Automatic Byparr Scaler - Monitors channel count and auto-scales
# Runs every 5 minutes via cron

CHANNELS_FILE="/home/ubuntu/goondvr/conf/channels.json"
APP_DIR="/home/ubuntu/goondvr"
LOG_FILE="/home/ubuntu/goondvr/auto-scaler.log"

# Log with timestamp
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Count active channels
ACTIVE_CHANNELS=$(jq '[.[] | select(.is_paused == false)] | length' "$CHANNELS_FILE" 2>/dev/null)

if [ -z "$ACTIVE_CHANNELS" ] || [ "$ACTIVE_CHANNELS" = "null" ]; then
    log "ERROR: Could not read channels.json"
    exit 1
fi

# Calculate needed instances based on available memory
# c7i-flex.large (4GB RAM): Max 6 instances
# c7i-flex.xlarge (8GB RAM): Max 12 instances
# t3.small (2GB RAM): Max 3 instances
# t2.micro (1GB RAM): Max 1 instance

# Get total memory in MB
TOTAL_MEM=$(free -m | awk '/^Mem:/{print $2}')

# Determine max instances based on memory
if [ $TOTAL_MEM -lt 1500 ]; then
    # t2.micro (1GB) - Very limited
    MAX_INSTANCES=1
    CHANNELS_PER_INSTANCE=10
elif [ $TOTAL_MEM -lt 3000 ]; then
    # t3.small (2GB)
    MAX_INSTANCES=3
    CHANNELS_PER_INSTANCE=10
elif [ $TOTAL_MEM -lt 6000 ]; then
    # c7i-flex.large (4GB) - FREE TIER!
    MAX_INSTANCES=6
    CHANNELS_PER_INSTANCE=10
elif [ $TOTAL_MEM -lt 12000 ]; then
    # c7i-flex.xlarge (8GB)
    MAX_INSTANCES=12
    CHANNELS_PER_INSTANCE=10
else
    # Larger instances (16GB+)
    MAX_INSTANCES=20
    CHANNELS_PER_INSTANCE=10
fi

# Calculate needed instances (1 per 10 channels)
NEEDED_INSTANCES=$(( (ACTIVE_CHANNELS + CHANNELS_PER_INSTANCE - 1) / CHANNELS_PER_INSTANCE ))

# Enforce minimum and maximum
if [ $NEEDED_INSTANCES -lt 1 ]; then
    NEEDED_INSTANCES=1
fi
if [ $NEEDED_INSTANCES -gt $MAX_INSTANCES ]; then
    NEEDED_INSTANCES=$MAX_INSTANCES
    log "⚠️  WARNING: Limited to $MAX_INSTANCES instances due to ${TOTAL_MEM}MB RAM"
    log "⚠️  Consider upgrading instance type for more channels"
fi

# Get current instance count
CURRENT_INSTANCES=$(cd "$APP_DIR" && sudo docker compose ps byparr --format json 2>/dev/null | jq -s 'length')

if [ -z "$CURRENT_INSTANCES" ]; then
    CURRENT_INSTANCES=0
fi

# Check if scaling is needed
if [ $CURRENT_INSTANCES -eq $NEEDED_INSTANCES ]; then
    log "✅ Optimal scale: $CURRENT_INSTANCES instances for $ACTIVE_CHANNELS channels"
    exit 0
fi

# Scale if needed
log "📊 Active channels: $ACTIVE_CHANNELS"
log "🔧 Current instances: $CURRENT_INSTANCES"
log "🎯 Needed instances: $NEEDED_INSTANCES"
log "⚙️  Scaling Byparr from $CURRENT_INSTANCES to $NEEDED_INSTANCES instances..."

cd "$APP_DIR"
sudo docker compose up -d --scale byparr=$NEEDED_INSTANCES --no-recreate >> "$LOG_FILE" 2>&1

if [ $? -eq 0 ]; then
    log "✅ Successfully scaled to $NEEDED_INSTANCES instances"
    
    # Send Discord notification if webhook is configured
    DISCORD_WEBHOOK=$(jq -r '.discord_webhook_url // empty' "$APP_DIR/conf/settings.json" 2>/dev/null)
    if [ ! -z "$DISCORD_WEBHOOK" ]; then
        curl -X POST "$DISCORD_WEBHOOK" \
            -H "Content-Type: application/json" \
            -d "{\"content\":\"🔧 Auto-scaled Byparr: $CURRENT_INSTANCES → $NEEDED_INSTANCES instances ($ACTIVE_CHANNELS channels)\"}" \
            2>/dev/null
    fi
else
    log "❌ Failed to scale instances"
    exit 1
fi
