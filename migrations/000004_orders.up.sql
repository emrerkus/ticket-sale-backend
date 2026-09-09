-- Migration 000004 (UP): siparisler.
--
-- Akis: kullanici koltuklari HOLD eder -> bu hold'lari bir SIPARISE cevirir
-- (pending) -> odeme yapinca siparis 'paid' olur ve koltuklar 'sold'a gecer.
-- Odeme yapilmazsa siparis 'expired' olur, koltuklar geri birakilir (worker).

CREATE TABLE orders (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    status          text        NOT NULL DEFAULT 'pending',
    total_cents     bigint      NOT NULL DEFAULT 0,
    currency        text        NOT NULL DEFAULT 'TRY',

    -- Idempotency: ayni "Idempotency-Key" ile gelen ikinci istek YENI siparis
    -- olusturmaz, var olani doner. Cift tiklama / retry guvenligi.
    idempotency_key text,

    -- Bu ana kadar odeme gelmezse siparis expired olur (worker temizler).
    expires_at      timestamptz NOT NULL,

    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT orders_status_check
        CHECK (status IN ('pending', 'paid', 'cancelled', 'expired')),
    CONSTRAINT orders_total_check CHECK (total_cents >= 0)
);

-- idempotency_key KULLANICI BAZINDA benzersiz (NULL'a izin var). Ayni kullanici
-- ayni anahtarla tekrar denerse var olan siparisi alir; farkli kullanicilar
-- ayni anahtari (nadiren) kullanabilir, birbirini etkilemez.
CREATE UNIQUE INDEX orders_user_idempotency_uq
    ON orders (user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Worker: WHERE status='pending' AND expires_at < now()
CREATE INDEX orders_status_expires_idx ON orders (status, expires_at);
CREATE INDEX orders_user_id_idx ON orders (user_id);

CREATE TABLE order_items (
    id               uuid   PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         uuid   NOT NULL REFERENCES orders(id)       ON DELETE CASCADE,
    event_seat_id    uuid   NOT NULL REFERENCES event_seats(id)  ON DELETE RESTRICT,

    -- Fiyati siparis aninda SABITLE (ticket_type fiyati sonradan degisirse
    -- kullanici odeyecegi tutar degismesin).
    unit_price_cents bigint NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT order_items_price_check CHECK (unit_price_cents >= 0)
);

CREATE INDEX order_items_order_id_idx ON order_items (order_id);
CREATE INDEX order_items_event_seat_id_idx ON order_items (event_seat_id);
