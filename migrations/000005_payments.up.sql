-- Migration 000005 (UP): odemeler.
--
-- Gercek hayatta odeme bir dis saglayiciya (iyzico, Stripe...) gider ve
-- sonuc bir webhook ile asenkron doner. Biz MOCK bir saglayici kullaniyoruz:
-- odeme senkron ve deterministik (test edilebilir olsun diye). Yapi yine de
-- gercekcile: her odeme denemesi bir kayit, durumu pending -> succeeded/failed.

CREATE TABLE payments (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id     uuid        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,

    provider     text        NOT NULL DEFAULT 'mock',
    provider_ref text        NOT NULL DEFAULT '',   -- saglayicinin islem no'su

    amount_cents bigint      NOT NULL,
    currency     text        NOT NULL DEFAULT 'TRY',

    status       text        NOT NULL DEFAULT 'pending',

    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT payments_status_check CHECK (status IN ('pending', 'succeeded', 'failed')),
    CONSTRAINT payments_amount_check CHECK (amount_cents >= 0)
);

CREATE INDEX payments_order_id_idx ON payments (order_id);
