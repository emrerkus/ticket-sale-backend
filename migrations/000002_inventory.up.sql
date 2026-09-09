-- Migration 000002 (UP): koltuklar, bilet tipleri, envanter.
--
-- SQL yukaridan asagi calisir: bir tablo, REFERENCES verdigi tablodan SONRA
-- gelmeli. Sira: seats -> ticket_types -> event_seats.

-- =====================================================================
-- seats — mekanin fiziksel koltuklari (etkinlikten bagimsiz)
-- =====================================================================
-- row_label / seat_number TEXT'tir ("12A" gibi degerler mumkun olsun diye).
-- Ayni mekanda ayni (section, row_label, seat_number) tekrar edemez.
CREATE TABLE seats (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id        uuid        NOT NULL REFERENCES venues(id)  ON DELETE CASCADE,
    section         text        NOT NULL,
    row_label       text        NOT NULL,
    seat_number     text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),

    -- Veri Bütünlüğü: Aynı mekanda aynı koltuk koordinatları tekrar edemez.
    CONSTRAINT seats_unique_position UNIQUE (venue_id, section, row_label, seat_number)
);


-- =====================================================================
-- ticket_types — bir etkinligin fiyat kategorileri ("VIP", "Tam", "Ogrenci")
-- =====================================================================
-- price_cents: fiyat EN KUCUK PARA BIRIMINDE (kurus), tam sayi. Asla float:
-- 0.1 + 0.2 != 0.3 (IEEE 754). Oran gerekince NUMERIC'e cevir, yuvarla, geri sakla.
CREATE TABLE ticket_types (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id    uuid        NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name        text        NOT NULL,
    
    -- Para her zaman en küçük birim (kuruş/cent) ve tam sayı olarak saklanır.
    price_cents bigint      NOT NULL,
    currency    text        NOT NULL DEFAULT 'TRY',
    created_at  timestamptz NOT NULL DEFAULT now(),

    -- Veri Bütünlüğü Kısıtlamaları
    CONSTRAINT ticket_types_price_check CHECK (price_cents >= 0),
    CONSTRAINT ticket_types_currency_check CHECK (char_length(currency) = 3),
    
    -- Bir etkinlikte "VIP" adında iki farklı bilet tipi olamaz.
    CONSTRAINT ticket_types_unique_name_per_event UNIQUE (event_id, name)
);

-- =====================================================================
-- event_seats — ENVANTER: bir etkinlikteki satilabilir tek bir koltuk
-- =====================================================================
CREATE TABLE event_seats (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Etkinlik silinirse envanteri de gitsin.
    event_id       uuid        NOT NULL REFERENCES events(id)       ON DELETE CASCADE,

    -- Fiziksel koltuk. RESTRICT: envanterde kullanilan bir koltuk silinemesin.
    seat_id        uuid        NOT NULL REFERENCES seats(id)        ON DELETE RESTRICT,

    -- Bu koltuk bu etkinlikte hangi fiyattan satiliyor.
    ticket_type_id uuid        NOT NULL REFERENCES ticket_types(id) ON DELETE RESTRICT,

    -- === Koltugun durumu — projenin en kritik kolonu ===
    -- available: bos, satin alinabilir
    -- held:      birileri sepetine aldi, gecici olarak rezerve (held_until'a kadar)
    -- sold:      odendi, kesin satildi
    status         text        NOT NULL DEFAULT 'available',

    -- status='held' iken hold'un dolacagi an. Diger durumlarda NULL.
    -- Worker: WHERE status='held' AND held_until < now() -> serbest birak.
    held_until     timestamptz,

    -- Optimistic locking sayaci. Her UPDATE'te uygulama bunu +1 yapar.
    version        integer     NOT NULL DEFAULT 0,

    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    -- Bir fiziksel koltuk, bir etkinlikte YALNIZCA BIR KEZ envantere girebilir.
    -- Bu UNIQUE, "ayni koltugu iki envanter satirina koyup iki kere satma" hatasini
    -- veritabani seviyesinde imkansiz kilar.
    CONSTRAINT event_seats_unique_seat_per_event UNIQUE (event_id, seat_id),

    -- status yalnizca bu uc degerden biri olabilir.
    CONSTRAINT event_seats_status_check CHECK (status IN ('available', 'held', 'sold')),

    -- Tutarlilik: held ise held_until dolu olmali; degilse bos olmali.
    CONSTRAINT event_seats_held_until_check CHECK (
        (status = 'held'  AND held_until IS NOT NULL) OR
        (status <> 'held' AND held_until IS NULL)
    )
);

-- Index'ler (Postgres FK kolonlarini otomatik indekslemez):
--   * (event_id, status): "bu etkinlikteki musait koltuklar" sorgusu.
--   * (held_until) PARTIAL WHERE status='held': worker'in suresi gecen hold taramasi;
--     satirlarin cogu 'available'/'sold' oldugu icin index kucuk kalir.
--   * seat_id / ticket_type_id: FK join'leri.
CREATE INDEX event_seats_event_status_idx  ON event_seats (event_id, status);
CREATE INDEX event_seats_held_until_idx     ON event_seats (held_until) WHERE status = 'held';
CREATE INDEX event_seats_seat_id_idx        ON event_seats (seat_id);
CREATE INDEX event_seats_ticket_type_idx    ON event_seats (ticket_type_id);
