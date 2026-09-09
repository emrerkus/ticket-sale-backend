-- Migration 000003 (DOWN): held_by kolonunu ve kisitini geri al.
-- Ters sira: once constraint, sonra kolon.

ALTER TABLE event_seats DROP CONSTRAINT IF EXISTS event_seats_held_by_check;
ALTER TABLE event_seats DROP COLUMN IF EXISTS held_by;
