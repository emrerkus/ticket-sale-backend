-- Migration 000001 (UP): cekirdek katalog tablolari.
--
-- Kural: bu dosya BIR KEZ uygulanir ve bir daha DEGISTIRILMEZ. Semayi degistirmek
-- istersen YENI bir migration dosyasi eklersin (000002_...). Boylece her ortam
-- (lokal, prod, is arkadasinin makinesi) ayni adimlardan gecerek ayni semaya ulasir.

-- gen_random_uuid() fonksiyonu icin. Postgres 13+ ile cekirdekte var ama
-- IF NOT EXISTS ile garanti altina aliyoruz.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =====================================================================
-- users — bilet alan kisiler
-- =====================================================================
CREATE TABLE users (
    -- UUID PK tercihi (bigint yerine):
    --   + Tahmin edilemez: /users/1, /users/2 diye gezilemez (IDOR riskini azaltir).
    --   + ID'yi client tarafinda uretebilirsin, insert'ten once bilinir.
    --   - 16 byte (bigint 8 byte) -> index'ler biraz daha buyuk.
    --   - Rastgele olduklari icin insert sirasinda index'te "sicrama" olur (v7 UUID bunu cozer; ileride).
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),

    email         text        NOT NULL,

    -- ASLA duz sifre saklanmaz. Burada bcrypt hash'i durur (internal/auth).
    password_hash text        NOT NULL,

    -- Her zaman timestamptz (timestamp DEGIL). timestamptz UTC olarak saklar,
    -- okurken client saat dilimine cevrilir. "timestamp" saat dilimi bilgisini
    -- atar ve er ya da gec yaz saati / sunucu tasima kaynakli hata uretir.
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- email benzersiz OLMALI ama buyuk/kucuk harf duyarsiz: "Ali@x.com" == "ali@x.com".
-- Bunu bir UNIQUE constraint yerine lower(email) uzerinde UNIQUE INDEX ile yapiyoruz.
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

-- =====================================================================
-- venues — etkinlik mekanlari (fiziksel, etkinlikten bagimsiz)
-- =====================================================================
-- Bir mekanin fiziksel koltuklari ("seats" tablosu, migration 000002) bir kez
-- tanimlanir; ayni mekandaki her etkinlik bu koltuklari yeniden kullanir.
CREATE TABLE venues (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL,
    address    text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- =====================================================================
-- events — etkinlikler
-- =====================================================================
-- status: 'draft' | 'published' | 'cancelled' (CHECK ile sinirli).
-- Bilet satis penceresi (sales_start_at .. sales_end_at) ve baslama ani
-- ayri tutulur; CHECK'ler tutarli bir zaman sirasini garanti eder.
CREATE TABLE events (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id         uuid        NOT NULL REFERENCES venues(id) ON DELETE RESTRICT,
    title            text        NOT NULL,
    description      text        NOT NULL DEFAULT '',
    starts_at        timestamptz NOT NULL,
    sales_start_at   timestamptz NOT NULL,
    sales_end_at     timestamptz NOT NULL,
    status           text        NOT NULL DEFAULT 'draft',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    -- Veri Bütünlüğü Kısıtlamaları (Data Integrity Constraints)
    CONSTRAINT events_status_check 
        CHECK (status IN ('draft', 'published', 'cancelled')),
    
    CONSTRAINT events_sales_window_check 
        CHECK (sales_start_at < sales_end_at),
    
    CONSTRAINT events_sales_end_before_start_check 
        CHECK (sales_end_at <= starts_at)
);

-- Postgres foreign key'ler için otomatik index OLUŞTURMAZ.
-- Bir mekanın (venue) etkinliklerini çekerken tam tablo taraması (seq scan)
-- yapmasını engellemek ve performansı korumak için bu index şarttır.
CREATE INDEX events_venue_id_idx ON events (venue_id);

-- Bileşik (Composite) Index:
-- Uygulama çoğu zaman "status = 'published'" ve "starts_at > now()" 
-- şeklinde filtrelemeler yapacaktır. Bu bileşik index, 
-- bu tür okuma sorgularını inanılmaz derecede hızlandırır.
CREATE INDEX events_status_starts_at_idx ON events (status, starts_at);
