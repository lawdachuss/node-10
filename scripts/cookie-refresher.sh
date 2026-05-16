#!/bin/sh
apk add --no-cache curl jq
echo '[COOKIE] Starting Cloudflare cookie refresher...'
until curl -fsS --max-time 5 http://byparr-lb/backend-health > /dev/null 2>&1; do
  echo '[COOKIE] Waiting for Byparr backend...'
  sleep 5
done

while true; do
  echo '[COOKIE] Getting cf_clearance from Byparr...'
  RESPONSE=$(curl -sS --fail --retry 3 --retry-all-errors --retry-delay 5 --max-time 240 -X POST http://byparr-lb/v1 \
    -H 'Content-Type: application/json' \
    -d '{"cmd":"request.get","url":"https://chaturbate.com","maxTimeout":180000}')
  CF_COOKIE=$(echo "$RESPONSE" | jq -r '.solution.cookies[] | select(.name=="cf_clearance") | .name + "=" + .value' 2>/dev/null)
  CF_USER_AGENT=$(echo "$RESPONSE" | jq -r '.solution.userAgent // empty' 2>/dev/null)
  if [ -n "$CF_COOKIE" ]; then
    echo "[COOKIE] Refreshed cf_clearance"
    if [ -n "$CF_USER_AGENT" ]; then
      body=$(jq -n --arg cookies "$CF_COOKIE" --arg ua "$CF_USER_AGENT" '{cookies:$cookies, user_agent:$ua}')
    else
      body=$(jq -n --arg cookies "$CF_COOKIE" '{cookies:$cookies}')
    fi
    curl -s --max-time 10 -X POST http://chaturbate-dvr:8080/update_config \
      -H 'Content-Type: application/json' \
      -d "$body" > /dev/null 2>&1
    echo '[COOKIE] Pushed to chaturbate-dvr'
  else
    echo '[COOKIE] Failed to get cf_clearance, retrying...'
  fi
  sleep 1800
done
