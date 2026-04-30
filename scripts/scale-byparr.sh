#!/bin/bash
# Auto-scale Byparr instances based on channel count

CHANNELS_FILE="conf/channels.json"
COMPOSE_FILE="docker-compose.yml"

# Count active channels (not paused)
CHANNEL_COUNT=$(jq '[.[] | select(.is_paused == false)] | length' "$CHANNELS_FILE")

# Calculate needed Byparr instances (1 instance per 10 channels, minimum 3, maximum 20)
NEEDED_INSTANCES=$(( (CHANNEL_COUNT + 9) / 10 ))
if [ $NEEDED_INSTANCES -lt 3 ]; then
    NEEDED_INSTANCES=3
fi
if [ $NEEDED_INSTANCES -gt 20 ]; then
    NEEDED_INSTANCES=20
fi

echo "📊 Active channels: $CHANNEL_COUNT"
echo "🔧 Needed Byparr instances: $NEEDED_INSTANCES"

# Scale the byparr service
docker compose up -d --scale byparr=$NEEDED_INSTANCES --no-recreate

echo "✅ Scaled to $NEEDED_INSTANCES Byparr instances"
