-- Migration 000001 (DOWN): up'ta yapilanlari TERS sirada geri al.
-- Sira onemli: FK bagimliligi olan tablo once dusurulur (events -> venues).

DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS venues;
DROP TABLE IF EXISTS users; -- index/constraint'ler tabloyla birlikte gider

-- pgcrypto extension'i birakiliyor (baska sey kullaniyor olabilir).
