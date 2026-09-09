-- Migration 000002 (DOWN): ters sira -- event_seats once (seats + ticket_types'a bagli).
DROP TABLE IF EXISTS event_seats;
DROP TABLE IF EXISTS ticket_types;
DROP TABLE IF EXISTS seats;