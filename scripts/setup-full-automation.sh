#!/bin/bash
# Setup FULL AUTOMATION - No manual intervention needed!

echo "🤖 Setting up FULL AUTOMATION..."
echo "=================================="
echo ""

APP_DIR="/home/ubuntu/goondvr"

# Make all scripts executable
chmod +x "$APP_DIR/scripts/"*.sh

# Remove existing goondvr cron jobs to avoid duplicates
crontab -l 2>/dev/null | grep -v goondvr | crontab -

# Add all automation cron jobs
echo "📝 Installing cron jobs..."

# 1. Auto-scaler - Every 5 minutes
(crontab -l 2>/dev/null; echo "# Auto-scale Byparr based on channel count") | crontab -
(crontab -l 2>/dev/null; echo "*/5 * * * * $APP_DIR/scripts/auto-scaler.sh >> $APP_DIR/auto-scaler.log 2>&1") | crontab -

# 2. Docker cleanup - Daily at 3 AM
(crontab -l 2>/dev/null; echo "# Clean Docker cache daily") | crontab -
(crontab -l 2>/dev/null; echo "0 3 * * * $APP_DIR/scripts/cleanup-disk.sh >> $APP_DIR/cleanup.log 2>&1") | crontab -

# 3. Auto-upload - Every 2 hours
(crontab -l 2>/dev/null; echo "# Auto-upload recordings every 2 hours") | crontab -
(crontab -l 2>/dev/null; echo "0 */2 * * * $APP_DIR/scripts/auto-upload-and-cleanup.sh >> $APP_DIR/upload-cleanup.log 2>&1") | crontab -

# 4. Emergency cleanup - Every 15 minutes if disk > 85%
(crontab -l 2>/dev/null; echo "# Emergency cleanup when disk > 85%") | crontab -
(crontab -l 2>/dev/null; echo "*/15 * * * * [ \$(df / | tail -1 | awk '{print \$5}' | sed 's/%//') -gt 85 ] && $APP_DIR/scripts/auto-upload-and-cleanup.sh >> $APP_DIR/emergency-cleanup.log 2>&1") | crontab -

# 5. Health check - Every 10 minutes
(crontab -l 2>/dev/null; echo "# Health check and auto-restart if needed") | crontab -
(crontab -l 2>/dev/null; echo "*/10 * * * * cd $APP_DIR && sudo docker compose ps | grep -q 'unhealthy' && sudo docker compose restart >> $APP_DIR/health-check.log 2>&1") | crontab -

echo ""
echo "✅ FULL AUTOMATION INSTALLED!"
echo "=============================="
echo ""
echo "Automatic Tasks:"
echo "  🔧 Every 5 minutes  - Auto-scale Byparr (based on channels)"
echo "  🧹 Every 2 hours    - Upload recordings & cleanup"
echo "  🗑️  Daily at 3 AM    - Clean Docker cache"
echo "  🚨 Every 15 minutes - Emergency cleanup (if disk > 85%)"
echo "  💚 Every 10 minutes - Health check & auto-restart"
echo ""
echo "Logs:"
echo "  • $APP_DIR/auto-scaler.log"
echo "  • $APP_DIR/cleanup.log"
echo "  • $APP_DIR/upload-cleanup.log"
echo "  • $APP_DIR/emergency-cleanup.log"
echo "  • $APP_DIR/health-check.log"
echo ""
echo "View cron jobs:"
echo "  crontab -l"
echo ""
echo "🎉 Your system is now FULLY AUTOMATIC!"
echo "   Just add channels and everything else happens automatically!"
