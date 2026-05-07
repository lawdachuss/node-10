# 🚀 Quick Start Guide

## What Just Got Fixed

✅ **Tunnel verification** - Now properly checks if tunnel is accessible
✅ **Supabase compatibility** - Fixed schema mismatch errors  
✅ **Better error handling** - Clearer messages and fallback logic
✅ **Workflow scheduling** - No more "higher priority request" cancellations

## 📋 What You See in Logs

### ✅ Good Signs
```
✅ App confirmed ready
✅ Tunnel URL found: https://xxxxx.trycloudflare.com
✅ Tunnel is VERIFIED ACCESSIBLE (HTTP 200)
✅ Tunnel URL saved to Supabase (HTTP 201)
```

### ⚠️ Warning Signs (Usually OK)
```
⏳ Connection timeout or DNS not resolved yet...
⚠️  HTTP 502 (waiting for 200...)
⏳ PROPAGATING (may take 1-2 minutes)
```
**What to do:** Wait 1-2 minutes, the tunnel is still setting up

### ❌ Error Signs
```
❌ App is not responding on port 8080!
❌ No tunnel URL found at all
❌ Failed to save URL (HTTP 400)
```
**What to do:** Check the logs for the specific error

## 🎯 How to Access Your UI

### Method 1: Use the Script (Easiest)
```powershell
.\scripts\open-ui.ps1
```

This will:
1. Fetch all recent tunnel URLs from Supabase
2. Test each one for accessibility
3. Open the first working one in your browser

### Method 2: Check GitHub Actions
1. Go to: https://github.com/vasud3v/record/actions
2. Click the running workflow
3. Look for the green box with the tunnel URL
4. Copy and paste in your browser

### Method 3: Query Supabase Directly
```powershell
# Set credentials
$env:SUPABASE_URL = "your-url"
$env:SUPABASE_API_KEY = "your-key"

# Get tunnel URL
.\scripts\get-tunnel-url.ps1
```

## 🔧 Current Workflow Behavior

### What Happens Now:
1. **App starts** - Verifies it's responding on port 8080
2. **Tunnel created** - Cloudflare Tunnel is started
3. **Verification** - Tests tunnel accessibility (HTTP 200)
4. **Retry logic** - If verification fails, retries up to 3 times
5. **Fallback** - If all retries fail, proceeds anyway (DNS propagation)
6. **Saves to Supabase** - Stores URL for easy retrieval
7. **Monitoring** - Checks tunnel health every 2 minutes
8. **Auto-recreation** - Recreates tunnel if it becomes inaccessible

### Timing:
- **App startup:** ~30-60 seconds
- **Tunnel creation:** ~10-30 seconds  
- **Verification:** ~20-100 seconds (with retries)
- **Total:** ~1-3 minutes from workflow start to accessible UI

## 🐛 Troubleshooting

### "HTTP code 000000" or "Connection timeout"
**Cause:** DNS not resolved yet or network issue  
**Solution:** Wait 1-2 minutes, this is normal during tunnel setup

### "HTTP 400" when saving to Supabase
**Cause:** Schema mismatch (now fixed!)  
**Solution:** Already fixed in latest push

### "HTTP 502/503/504" from tunnel
**Cause:** Tunnel exists but backend app not responding  
**Solution:** Wait 1-2 minutes for app to fully start

### Tunnel URL not accessible
**Cause:** Cloudflare DNS propagation delay  
**Solution:** Wait 2-3 minutes after tunnel creation

### Workflow keeps canceling
**Cause:** Old concurrency settings (now fixed!)  
**Solution:** Already fixed - workflows won't cancel each other

## 📊 Check Workflow Status

```powershell
# Requires GitHub CLI (gh)
.\scripts\check-github-actions-status.ps1
```

Or visit: https://github.com/vasud3v/record/actions

## 🎛️ Manual Workflow Trigger

1. Go to: https://github.com/vasud3v/record/actions
2. Click "24/7 Recorder"
3. Click "Run workflow"
4. Choose duration (default: 5 hours)
5. Click "Run workflow"

## 📝 Important Notes

- **Tunnel URLs are temporary** - They expire when the workflow ends
- **Workflow auto-restarts** - Triggers itself for continuous operation
- **Maximum duration** - 6 hours per run (GitHub Actions limit)
- **No authentication needed** - Cloudflare Tunnel is free and requires no setup
- **Supabase required** - For storing and retrieving tunnel URLs

## 🔐 Required Secrets

Make sure these are set in GitHub Settings → Secrets:

- `SUPABASE_URL` - Your Supabase project URL
- `SUPABASE_API_KEY` - Your Supabase API key

## ✅ What's Working Now

- ✅ Tunnel accessibility verification
- ✅ Automatic retry on failure
- ✅ Health monitoring and auto-recreation
- ✅ Proper Supabase schema compatibility
- ✅ No workflow cancellation issues
- ✅ Clear error messages and logging
- ✅ Fallback for DNS propagation delays

## 🆘 Still Having Issues?

1. **Check the workflow logs** - Most issues are visible there
2. **Run diagnostics:**
   ```powershell
   .\scripts\diagnose-tunnel.ps1 <tunnel-url>
   ```
3. **Verify Supabase:**
   ```powershell
   Invoke-RestMethod -Uri "$env:SUPABASE_URL/rest/v1/tunnel_sessions?select=*&limit=1" -Headers @{"apikey"=$env:SUPABASE_API_KEY}
   ```
4. **Check if workflow is running:**
   ```powershell
   .\scripts\check-github-actions-status.ps1
   ```

## 📚 Full Documentation

See `TUNNEL_ACCESS_GUIDE.md` for complete documentation.
