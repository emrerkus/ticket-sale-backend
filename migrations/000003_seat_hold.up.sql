-- Migration 000003 (UP): hold'a "kim tutuyor" bilgisini ekle.
--
-- held_by: hold'u yapan giris yapmis kullanicinin id'si. Kullanimi:
--   * hold iptalinde "sadece sahibi iptal edebilir" kontrolu (users(id)'ye FK yok,
--     cunku token dogrulamasi zaten uygulama katmaninda yapiliyor).
--   * checkout'ta "bu koltuk gercekten bu kullanicida held mi" dogrulamasi.

ALTER TABLE event_seats ADD COLUMN held_by uuid;

-- Tutarlilik: status='held' ISE held_by dolu olmali; degilse bos olmali.
-- (status = 'held')  ve  (held_by IS NOT NULL)  ayni boolean degeri vermeli.
-- Not: seed sonrasi tum satirlar 'available'/NULL oldugu icin kural saglanir.
ALTER TABLE event_seats
    ADD CONSTRAINT event_seats_held_by_check
    CHECK ( (status = 'held') = (held_by IS NOT NULL) );
