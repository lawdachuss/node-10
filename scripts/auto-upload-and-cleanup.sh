#!/bin/bash
# Auto-upload recordings to cloud and cleanup local files
# Runs automatically to prevent disk from filling up

VIDEOS_DIR="/home/ubuntu/goondvr/videos"
UPLOAD_SCRIPT="/home/ubuntu/goondvr/upload_to_gofile.py"
MIN_FREE_SPACE_GB=5

echo "📤 Auto-Upload and Cleanup Script"
echo "=================================="
echo ""

# Check current free space
FREE_SPACE_GB=$(df / | tail -1 | awk '{print $4}' | awk '{print int($1/1024/1024)}')
echo "💾 Free space: ${FREE_SPACE_GB} GB"

if [ $FREE_SPACE_GB -lt $MIN_FREE_SPACE_GB ]; then
    echo "⚠️  Low disk space! Starting emergency cleanup..."
    
    # Find completed recordings
    cd "$VIDEOS_DIR"
    COMPLETED_FILES=$(find . -name "*.mp4" -type f -mmin +60 | sort)
    
    if [ -z "$COMPLETED_FILES" ]; then
        echo "   No completed recordings to upload"
    else
        echo "   Found $(echo "$COMPLETED_FILES" | wc -l) completed recordings"
        
        # Upload each file
        for file in $COMPLETED_FILES; do
            echo ""
            echo "📤 Uploading: $file"
            
            # Upload to cloud (GoFile, Supabase, etc.)
            python3 "$UPLOAD_SCRIPT" "$file"
            
            if [ $? -eq 0 ]; then
                echo "   ✅ Upload successful, deleting local file..."
                rm "$file"
                
                # Check if we have enough space now
                FREE_SPACE_GB=$(df / | tail -1 | awk '{print $4}' | awk '{print int($1/1024/1024)}')
                if [ $FREE_SPACE_GB -ge $MIN_FREE_SPACE_GB ]; then
                    echo "   ✅ Sufficient space restored ($FREE_SPACE_GB GB free)"
                    break
                fi
            else
                echo "   ❌ Upload failed, keeping local file"
            fi
        done
    fi
else
    echo "✅ Sufficient disk space available"
fi

echo ""
echo "Final disk status:"
df -h /
