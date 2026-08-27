BEGIN;

-- Canonical FCM registrations always carry a device identifier.
-- The mobile runtime currently uses the FCM token itself as the device key,
-- so any legacy null/blank device_id rows are normalized to the token value
-- before the column is made required.
UPDATE fcm_tokens
SET device_id = token,
    updated_at = NOW()
WHERE device_id IS NULL
   OR BTRIM(device_id) = '';

ALTER TABLE fcm_tokens
    ALTER COLUMN device_id SET NOT NULL;

COMMIT;
