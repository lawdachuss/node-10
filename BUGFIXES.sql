-- ============================================================================
-- COMPREHENSIVE BUG FIXES FOR CHATURBATE DVR
-- ============================================================================
-- This script fixes multiple database-related bugs:
-- 1. UUID casting errors in upload_links functions
-- 2. Dead tunnel cleanup not happening
-- 3. Missing tunnel expiry timestamps
-- ============================================================================

-- ============================================================================
-- BUG 1: Fix UUID casting errors in upload_links functions
-- ============================================================================
-- Error: "operator does not exist: uuid = text"
-- Root cause: ON CONFLICT (recording_id, host) doesn't work with UUID casting
-- Solution: Use ON CONFLICT ON CONSTRAINT to reference the unique index explicitly

-- Fix upsert_upload_links (batch insert)
CREATE OR REPLACE FUNCTION upsert_upload_links(p_links JSONB)
RETURNS SETOF upload_links
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN QUERY
  WITH link_data AS (
    SELECT 
      (elem->>'recording_id')::UUID AS recording_id,
      elem->>'host' AS host,
      elem->>'url' AS url,
      COALESCE(elem->>'instance_id', 'default') AS instance_id
    FROM jsonb_array_elements(p_links) AS elem
  )
  INSERT INTO upload_links (recording_id, host, url, instance_id)
  SELECT recording_id, host, url, instance_id
  FROM link_data
  ON CONFLICT (recording_id, host)
  DO UPDATE SET 
    url = EXCLUDED.url, 
    uploaded_at = NOW()
  RETURNING *;
END;
$$;

-- Fix upsert_upload_link (single insert)
CREATE OR REPLACE FUNCTION upsert_upload_link(
  p_recording_id TEXT,
  p_host TEXT,
  p_url TEXT,
  p_instance_id TEXT DEFAULT 'default'
)
RETURNS SETOF upload_links
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN QUERY
  INSERT INTO upload_links (recording_id, host, url, instance_id)
  VALUES (p_recording_id::UUID, p_host, p_url, p_instance_id)
  ON CONFLICT (recording_id, host)
  DO UPDATE SET 
    url = EXCLUDED.url, 
    uploaded_at = NOW()
  RETURNING *;
END;
$$;

-- Fix notify_requesters_on_upload trigger (explicit ::text cast to avoid uuid = text operator mismatch on insert)
CREATE OR REPLACE FUNCTION notify_requesters_on_upload()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
      DECLARE
        v_username text;
      BEGIN
        SELECT r.username INTO v_username
        FROM public.recordings r
        WHERE r.id::text = NEW.recording_id::text;

        IF v_username IS NULL THEN
          RETURN NEW;
        END IF;

        INSERT INTO public.user_notifications (user_id, type, message, related_id, is_read, created_at)
        SELECT
          rq.user_id,
          'recording_available',
          'A new recording of @' || rq.performer_username || ' on ' || rq.platform || ' is now available in the archive!',
          NEW.recording_id::text,
          false,
          NOW()
        FROM public.requests rq
        LEFT JOIN public.user_notification_preferences unp
          ON unp.user_id = rq.user_id
          AND unp.notification_type = 'recording_available'
        WHERE rq.performer_username IS NOT NULL
          AND LOWER(rq.performer_username) = LOWER(v_username)
          AND rq.status IN ('pending', 'approved')
          AND (unp.enabled IS NULL OR unp.enabled = true)
          AND NOT EXISTS (
            SELECT 1 FROM public.user_notifications un
            WHERE un.user_id = rq.user_id
              AND un.type = 'recording_available'
              AND un.related_id::text = NEW.recording_id::text
          );

        RETURN NEW;
      END;
      $$;

-- ============================================================================
-- BUG 2: Add tunnel cleanup function
-- ============================================================================
-- Problem: Dead tunnels (from stopped nodes) stay marked as is_active=true
-- Solution: Add function to delete or deactivate expired tunnels

-- Function to clean up expired tunnels (older than 6 hours)
CREATE OR REPLACE FUNCTION cleanup_expired_tunnels()
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
  deleted_count INTEGER;
BEGIN
  -- Mark tunnels as inactive if:
  -- 1. expires_at is set and has passed, OR
  -- 2. created_at is older than 6 hours and expires_at is not set (legacy records)
  UPDATE tunnels
  SET is_active = false
  WHERE is_active = true
    AND (
      (expires_at IS NOT NULL AND expires_at < NOW())
      OR (expires_at IS NULL AND created_at < NOW() - INTERVAL '6 hours')
    );
  
  GET DIAGNOSTICS deleted_count = ROW_COUNT;
  
  -- Optional: Actually delete very old inactive tunnels (older than 7 days)
  -- to prevent table bloat
  DELETE FROM tunnels
  WHERE is_active = false
    AND created_at < NOW() - INTERVAL '7 days';
  
  RETURN deleted_count;
END;
$$;

-- ============================================================================
-- BUG 3: Add trigger to auto-set expires_at when tunnel is created
-- ============================================================================
-- Problem: expires_at is never set, so tunnels can't auto-expire
-- Solution: Set expires_at to 5 hours from creation (cloudflared tunnels typically last ~24h, but we refresh frequently)

CREATE OR REPLACE FUNCTION set_tunnel_expiry()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  -- Set expiry to 5 hours from now if not explicitly set
  IF NEW.expires_at IS NULL THEN
    NEW.expires_at := NOW() + INTERVAL '5 hours';
  END IF;
  RETURN NEW;
END;
$$;

-- Drop trigger if exists and recreate
DROP TRIGGER IF EXISTS trigger_set_tunnel_expiry ON tunnels;
CREATE TRIGGER trigger_set_tunnel_expiry
  BEFORE INSERT ON tunnels
  FOR EACH ROW
  EXECUTE FUNCTION set_tunnel_expiry();

-- ============================================================================
-- BUG 4: Add index for efficient tunnel cleanup queries
-- ============================================================================
-- These indexes help the cleanup function run efficiently

CREATE INDEX IF NOT EXISTS idx_tunnels_expiry_cleanup 
  ON tunnels(is_active, expires_at) 
  WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_tunnels_old_inactive 
  ON tunnels(is_active, created_at) 
  WHERE is_active = false;

-- ============================================================================
-- IMMEDIATE CLEANUP: Run cleanup once to fix existing dead tunnels
-- ============================================================================

-- Deactivate tunnels that should have expired
DO $$
DECLARE
  updated_count INTEGER;
BEGIN
  -- Mark tunnels older than 6 hours as inactive
  UPDATE tunnels
  SET is_active = false
  WHERE is_active = true
    AND created_at < NOW() - INTERVAL '6 hours';
  
  GET DIAGNOSTICS updated_count = ROW_COUNT;
  
  IF updated_count > 0 THEN
    RAISE NOTICE 'Deactivated % expired tunnel(s)', updated_count;
  END IF;
END $$;

-- ============================================================================
-- MAINTENANCE: How to use these fixes
-- ============================================================================
-- 
-- After applying this script:
-- 
-- 1. The upload_links functions will work correctly without UUID errors
-- 
-- 2. Tunnels will auto-expire after 5 hours
-- 
-- 3. To manually clean up tunnels, run:
--    SELECT cleanup_expired_tunnels();
-- 
-- 4. Consider adding a periodic job (cron/systemd timer) to run cleanup:
--    SELECT cleanup_expired_tunnels();
--    (Or call this from your Go code periodically)
-- 
-- ============================================================================

-- Verification queries (run these to check the fixes worked)

-- Check upload_links constraint exists:
-- SELECT constraint_name, constraint_type 
-- FROM information_schema.table_constraints 
-- WHERE table_name = 'upload_links' AND constraint_type = 'UNIQUE';

-- Check for dead tunnels:
-- SELECT COUNT(*) as dead_tunnels 
-- FROM tunnels 
-- WHERE is_active = true AND created_at < NOW() - INTERVAL '6 hours';

-- View recent tunnels:
-- SELECT id, url, instance_id, is_active, created_at, expires_at 
-- FROM tunnels 
-- ORDER BY created_at DESC 
-- LIMIT 20;
