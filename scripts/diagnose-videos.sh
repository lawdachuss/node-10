#!/bin/bash

# Diagnostic script to check video status
# Run this on EC2 to see what's happening with videos

echo "🔍 Video System Diagnostic"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

cd /home/ubuntu/goondvr || {
    echo "❌ Cannot find /home/ubuntu/goondvr directory!"
    exit 1
}

echo "📍 Current directory: $(pwd)"
echo ""

# Check 1: Video files
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1️⃣  Checking for video files..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -d "videos" ]; then
    echo "✅ videos/ directory exists"
    
    # Count video files
    VIDEO_COUNT=$(find videos -type f \( -name "*.mp4" -o -name "*.ts" -o -name "*.mkv" \) 2>/dev/null | wc -l)
    echo "📊 Total video files: $VIDEO_COUNT"
    
    if [ $VIDEO_COUNT -gt 0 ]; then
        echo ""
        echo "📁 Video files found:"
        find videos -type f \( -name "*.mp4" -o -name "*.ts" -o -name "*.mkv" \) -exec ls -lh {} \; | head -20
        
        echo ""
        echo "💾 Total size:"
        du -sh videos/
    else
        echo "⚠️  No video files found in videos/ directory"
    fi
    
    # Check completed directory
    if [ -d "videos/completed" ]; then
        COMPLETED_COUNT=$(find videos/completed -type f \( -name "*.mp4" -o -name "*.ts" -o -name "*.mkv" \) 2>/dev/null | wc -l)
        echo ""
        echo "📦 Completed videos: $COMPLETED_COUNT"
        if [ $COMPLETED_COUNT -gt 0 ]; then
            find videos/completed -type f \( -name "*.mp4" -o -name "*.ts" -o -name "*.mkv" \) -exec ls -lh {} \; | head -10
        fi
    fi
else
    echo "❌ videos/ directory does not exist!"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2️⃣  Checking database directory..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -d "database" ]; then
    echo "✅ database/ directory exists"
    
    # Count recordings.json files
    JSON_COUNT=$(find database -name "recordings.json" 2>/dev/null | wc -l)
    echo "📊 recordings.json files: $JSON_COUNT"
    
    if [ $JSON_COUNT -gt 0 ]; then
        echo ""
        echo "📄 Database files:"
        find database -name "recordings.json" -exec echo "  {}" \;
        
        echo ""
        echo "📋 Sample content from first database file:"
        FIRST_JSON=$(find database -name "recordings.json" 2>/dev/null | head -1)
        if [ -n "$FIRST_JSON" ]; then
            cat "$FIRST_JSON" | head -30
        fi
    else
        echo "⚠️  No recordings.json files found in database/"
    fi
    
    echo ""
    echo "📁 Database structure:"
    tree -L 3 database/ 2>/dev/null || find database -type d | head -20
else
    echo "❌ database/ directory does not exist!"
    echo "Creating database/ directory..."
    mkdir -p database
    echo "✅ Created database/ directory"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3️⃣  Checking settings..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -f "conf/settings.json" ]; then
    echo "✅ conf/settings.json exists"
    echo ""
    echo "📄 Upload settings:"
    grep -E "enable_gofile_upload|enable_supabase|turboviplay|voesx|streamtape" conf/settings.json || echo "No upload settings found"
else
    echo "❌ conf/settings.json does not exist!"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4️⃣  Checking channels..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -f "conf/channels.json" ]; then
    echo "✅ conf/channels.json exists"
    CHANNEL_COUNT=$(grep -o '"username"' conf/channels.json | wc -l)
    echo "📊 Configured channels: $CHANNEL_COUNT"
    
    if [ $CHANNEL_COUNT -gt 0 ]; then
        echo ""
        echo "📋 Channels:"
        grep '"username"' conf/channels.json | head -10
    fi
else
    echo "❌ conf/channels.json does not exist!"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "5️⃣  Checking Docker container..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if docker ps | grep -q goondvr; then
    echo "✅ goondvr container is running"
    
    echo ""
    echo "📊 Container stats:"
    docker stats goondvr --no-stream
    
    echo ""
    echo "📋 Recent logs (last 20 lines):"
    docker logs --tail 20 goondvr
    
    echo ""
    echo "🔍 Checking for upload-related logs:"
    docker logs goondvr 2>&1 | grep -i "upload\|gofile\|database" | tail -10 || echo "No upload logs found"
else
    echo "❌ goondvr container is not running!"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "6️⃣  Testing API endpoint..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo "Fetching /api/videos..."
curl -s http://localhost:8080/api/videos | jq '.' 2>/dev/null || curl -s http://localhost:8080/api/videos

echo ""
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Summary
if [ $VIDEO_COUNT -gt 0 ]; then
    echo "✅ Video files exist: $VIDEO_COUNT files"
else
    echo "⚠️  No video files found"
fi

if [ $JSON_COUNT -gt 0 ]; then
    echo "✅ Database records exist: $JSON_COUNT files"
else
    echo "⚠️  No database records found"
fi

if grep -q '"enable_gofile_upload": true' conf/settings.json 2>/dev/null; then
    echo "✅ GoFile upload is enabled"
else
    echo "⚠️  GoFile upload is disabled or not configured"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "💡 RECOMMENDATIONS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $VIDEO_COUNT -eq 0 ]; then
    echo "• No recordings yet - channels need to go live first"
    echo "• Check channel status in web UI"
fi

if [ $JSON_COUNT -eq 0 ] && [ $VIDEO_COUNT -gt 0 ]; then
    echo "• Videos exist but not in database"
    echo "• GoFile upload might not have been enabled when recording"
    echo "• Run: bash scripts/fix-gofile-enable.sh"
fi

if ! grep -q '"enable_gofile_upload": true' conf/settings.json 2>/dev/null; then
    echo "• GoFile upload is not enabled"
    echo "• Run: bash scripts/fix-gofile-enable.sh"
fi

echo ""
echo "✅ Diagnostic complete!"
