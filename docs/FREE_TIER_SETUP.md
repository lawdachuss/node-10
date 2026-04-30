# 🆓 AWS Free Tier Setup Guide

## ✅ What You Get for FREE

- **Instance:** t2.micro or t3.micro (1 vCPU, 1 GB RAM)
- **Hours:** 750 hours/month (run 24/7 for free!)
- **Storage:** 30 GB EBS volume
- **Data Transfer:** 15 GB/month outbound
- **Duration:** 12 months from AWS signup

## ⚠️ Free Tier Limitations

### Memory Constraints (1 GB RAM)
```
Total RAM: 1024 MB
├─ System/OS: ~200 MB
├─ Docker: ~100 MB
├─ Byparr (1 instance): ~400 MB
├─ GoOnDVR: ~200 MB
└─ Available: ~124 MB
```

**Realistic Capacity:**
- ✅ **1 Byparr instance** = ~10 channels max
- ⚠️ **2 Byparr instances** = ~15 channels (tight!)
- ❌ **3+ instances** = Will crash (out of memory)

### Disk Space (30 GB)
- **OS + Docker:** ~5 GB
- **Available for recordings:** ~25 GB
- **1080p recording:** ~2-3 GB/hour
- **Storage time:** ~8-12 hours of recordings

**Solution:** Auto-upload to cloud storage (Gofile, Streamtape, etc.)

## 🚀 Step-by-Step Setup

### 1. Launch Free Tier EC2 Instance

```bash
# Go to AWS Console → EC2 → Launch Instance

Instance Settings:
├─ Name: goondvr-free
├─ AMI: Ubuntu Server 22.04 LTS (Free tier eligible)
├─ Instance type: t2.micro (Free tier eligible)
├─ Key pair: Create new or use existing
├─ Storage: 30 GB gp3 (Free tier eligible)
└─ Security Group:
    ├─ SSH (22) - Your IP only
    └─ HTTP (8080) - Your IP only (for web UI)
```

### 2. Connect to Your Instance

```powershell
# Windows PowerShell
ssh -i aws-secrets/aws-key.pem ubuntu@YOUR_EC2_IP
```

### 3. Install Docker

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add user to docker group
sudo usermod -aG docker ubuntu

# Install Docker Compose
sudo apt install docker-compose-plugin -y

# Logout and login again
exit
```

### 4. Deploy GoOnDVR

```bash
# SSH back in
ssh -i aws-secrets/aws-key.pem ubuntu@YOUR_EC2_IP

# Clone or upload your project
mkdir -p /home/ubuntu/goondvr
cd /home/ubuntu/goondvr

# Upload files from your local machine (run on Windows):
# scp -i aws-secrets/aws-key.pem -r * ubuntu@YOUR_EC2_IP:/home/ubuntu/goondvr/

# Build and start (FREE TIER: 1 Byparr instance)
sudo docker compose up -d --build

# Check status
sudo docker compose ps
```

### 5. Setup Auto-Cleanup (CRITICAL for 30 GB disk!)

```bash
# Make scripts executable
chmod +x /home/ubuntu/goondvr/scripts/*.sh

# Setup automatic cleanup
/home/ubuntu/goondvr/scripts/setup-full-automation.sh
```

## 📊 Free Tier Optimization

### Memory Management

**Current Setup (Optimized):**
```yaml
byparr: 1 instance × 400 MB = 400 MB
recorder: 1 instance × 200 MB = 200 MB
nginx: 1 instance × 50 MB = 50 MB
Total: ~650 MB (leaves ~350 MB for system)
```

**If You Need More Channels:**
```bash
# Scale to 2 Byparr instances (handles ~15 channels)
# WARNING: This uses ~850 MB, leaving only ~150 MB free!
sudo docker compose up -d --scale byparr=2 --no-recreate

# Monitor memory usage
free -h
sudo docker stats
```

### Disk Management

**Auto-Upload Strategy:**
```bash
# Upload recordings every 2 hours
0 */2 * * * /home/ubuntu/goondvr/scripts/auto-upload-and-cleanup.sh

# Emergency cleanup if disk > 80%
*/15 * * * * [ $(df / | tail -1 | awk '{print $5}' | sed 's/%//') -gt 80 ] && /home/ubuntu/goondvr/scripts/auto-upload-and-cleanup.sh
```

**Manual Cleanup:**
```bash
# Check disk usage
df -h

# Clean Docker cache
sudo docker system prune -af --volumes

# Delete old recordings
find /home/ubuntu/goondvr/videos -name "*.mp4" -mtime +1 -delete
```

## 🎯 Recommended Channel Limits

### Conservative (Stable)
- **Channels:** 5-8 channels
- **Byparr:** 1 instance
- **Memory:** ~60-70% usage
- **Success Rate:** 70-85%

### Moderate (Balanced)
- **Channels:** 10-12 channels
- **Byparr:** 1 instance
- **Memory:** ~75-85% usage
- **Success Rate:** 65-80%

### Aggressive (Risky)
- **Channels:** 15-18 channels
- **Byparr:** 2 instances
- **Memory:** ~85-95% usage
- **Success Rate:** 60-75%
- **Risk:** May crash if all channels go live simultaneously

## 💰 When to Upgrade from Free Tier

### Signs You Need More Resources:

1. **Out of Memory Errors**
   ```
   Error: Cannot allocate memory
   Container "byparr" exited (137)
   ```
   → Upgrade to **t3.small** (2 GB RAM) - $15/month

2. **Disk Full Constantly**
   ```
   No space left on device
   ```
   → Add EBS volume or upgrade to 50 GB - $5/month

3. **Too Many Channels**
   ```
   > 15 channels active
   ```
   → Upgrade to **t3.medium** (4 GB RAM) - $30/month

## 🔧 Monitoring Commands

### Check Memory Usage
```bash
# Overall memory
free -h

# Per container
sudo docker stats

# If memory > 90%, restart to free up
sudo docker compose restart
```

### Check Disk Usage
```bash
# Disk space
df -h

# Docker disk usage
sudo docker system df

# Largest files
du -h /home/ubuntu/goondvr/videos | sort -rh | head -10
```

### Check Container Health
```bash
# All containers
sudo docker compose ps

# Logs
sudo docker compose logs recorder --tail=50
sudo docker compose logs byparr --tail=50

# Restart if unhealthy
sudo docker compose restart
```

## 📈 Scaling Path

### Free Tier (Current)
```
Cost: $0/month
RAM: 1 GB
Channels: ~10 channels
Byparr: 1 instance
```

### t3.small (First Upgrade)
```
Cost: ~$15/month
RAM: 2 GB
Channels: ~30 channels
Byparr: 3 instances
```

### t3.medium (Growth)
```
Cost: ~$30/month
RAM: 4 GB
Channels: ~60 channels
Byparr: 6 instances
```

### t3.large (Scale)
```
Cost: ~$60/month
RAM: 8 GB
Channels: ~120 channels
Byparr: 12 instances
```

## 🎁 Free Tier Tips

### Maximize Your Free Hours
- Free tier = 750 hours/month
- 1 instance 24/7 = 720 hours (within limit!)
- Can run continuously for 12 months

### Avoid Extra Charges
1. **Stop instance when not needed** (saves hours)
2. **Use 30 GB storage max** (free tier limit)
3. **Monitor data transfer** (15 GB/month free)
4. **Delete old snapshots** (not free!)
5. **Use CloudWatch free tier** (10 metrics)

### Cost Alerts
```bash
# Set up billing alerts in AWS Console:
1. Go to Billing Dashboard
2. Set alert for $1 (catches any charges)
3. Get email if you exceed free tier
```

## 🚨 Common Issues

### Out of Memory
```bash
# Symptoms: Containers crash, system slow
# Solution 1: Reduce to 1 Byparr instance
sudo docker compose up -d --scale byparr=1 --no-recreate

# Solution 2: Restart to free memory
sudo docker compose restart

# Solution 3: Enable swap (temporary fix)
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

### Disk Full
```bash
# Emergency cleanup
sudo docker system prune -af --volumes
find /home/ubuntu/goondvr/videos -name "*.mp4" -delete

# Check what's using space
du -sh /home/ubuntu/goondvr/*
sudo docker system df
```

### Byparr Crashes
```bash
# Check logs
sudo docker compose logs byparr --tail=100

# Restart
sudo docker compose restart byparr

# If still failing, reduce memory pressure
sudo docker compose up -d --scale byparr=1 --no-recreate
```

## ✅ Free Tier Checklist

- [ ] Launch t2.micro instance (free tier eligible)
- [ ] Use 30 GB storage (free tier limit)
- [ ] Configure security group (SSH + 8080)
- [ ] Install Docker & Docker Compose
- [ ] Deploy with 1 Byparr instance
- [ ] Setup auto-cleanup cron jobs
- [ ] Configure auto-upload to cloud storage
- [ ] Set AWS billing alerts
- [ ] Monitor memory usage daily
- [ ] Start with 5-8 channels, scale gradually

## 🎉 You're Ready!

Your free tier setup can handle:
- ✅ 5-10 channels comfortably
- ✅ 24/7 operation for 12 months
- ✅ Automatic cleanup & uploads
- ✅ $0/month cost

**When you outgrow free tier, upgrade to t3.small for $15/month!**

---

## Quick Commands Reference

```bash
# Start everything
sudo docker compose up -d

# Check status
sudo docker compose ps

# View logs
sudo docker compose logs recorder --tail=50

# Check memory
free -h && sudo docker stats --no-stream

# Check disk
df -h

# Restart if issues
sudo docker compose restart

# Scale Byparr (if you have RAM)
sudo docker compose up -d --scale byparr=2 --no-recreate

# Emergency cleanup
sudo docker system prune -af && find videos -name "*.mp4" -mtime +1 -delete
```

**Need help?** Check the logs and monitor memory/disk usage!
