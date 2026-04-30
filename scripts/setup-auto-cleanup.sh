#!/bin/bash
# Setup automatic disk cleanup cron jobs

echo "⚙️  Setting up automatic disk cleanup..."
echo ""

# Make scripts executable
chmod +x /home/ubuntu/goondvr/scripts/cleanup-disk.sh
chmod +x /home/ubuntu/goondvr/scripts/auto-upload-and-cleanup.sh

# Add cron jobs
(crontab -l 2>/dev/null; echo "# Auto cleanup Docker cache daily at 3 AM") | crontab -
(crontab -l 2>/dev/null; echo "0 3 * * * /home/ubuntu/goondvr/scripts/cleanup-disk.sh >> /home/ubuntu/goondvr/cleanup.log 2>&1") | crontab -

(crontab -l 2>/dev/null; echo "# Auto upload and cleanup every 6 hours") | crontab -
(crontab -l 2>/dev/null; echo "0 */6 * * * /home/ubuntu/goondvr/scripts/auto-upload-and-cleanup.sh >> /home/ubuntu/goondvr/upload-cleanup.log 2>&1") | crontab -

(crontab -l 2>/dev/null; echo "# Emergency cleanup when disk > 85%") | crontab -
(crontab -l 2>/dev/null; echo "*/30 * * * * [ \$(df / | tail -1 | awk '{print \$5}' | sed 's/%//') -gt 85 ] && /home/ubuntu/goondvr/scripts/auto-upload-and-cleanup.sh >> /home/ubuntu/goondvr/emergency-cleanup.log 2>&1") | crontab -

echo "✅ Cron jobs installed:"
echo ""
crontab -l | grep -v "^#" | grep goondvr
echo ""
echo "Cleanup schedule:"
echo "  • Daily at 3 AM: Docker cache cleanup"
echo "  • Every 6 hours: Auto-upload and cleanup recordings"
echo "  • Every 30 minutes: Emergency cleanup if disk > 85%"
echo ""
echo "Logs:"
echo "  • /home/ubuntu/goondvr/cleanup.log"
echo "  • /home/ubuntu/goondvr/upload-cleanup.log"
echo "  • /home/ubuntu/goondvr/emergency-cleanup.log"
