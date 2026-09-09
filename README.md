# ticketsale — koltuklu bilet satış uygulaması

Go + React + PostgreSQL + Redis ile yazılmış, koltuk seçimli (reserved seating) bilet
satış uygulaması. Bilet satışındaki backend / güvenlik / veritabanı incelikleri
(race condition, overselling, dağıtık kilit, transaction, idempotency, JWT) üzerine
kurulu bir öğrenme projesi.

## Mimari

```
[React SPA :5173]  ──HTTP/JSON──▶  [Go API :8080]  ──▶  [PostgreSQL :5433]   kalıcı gerçek:
      │                                  │                                   users, events, seats,
      │                                  │                                   event_seats, orders, payments
      │                                  └──────────────▶  [Redis :6379]      geçici: koltuk hold kilitleri
      │
      └── JWT (localStorage)        [Go worker]  ── süresi dolan hold/siparişleri temizler
```

**İki katmanlı gerçek:** Redis "şu an kim seçiyor" (hızlı, TTL'li, best-effort);
PostgreSQL "gerçekte ne oldu" (kalıcı, transaction'lı, kesin). Overselling'i önleyen
şey Redis değil, `event_seats.status` üzerindeki atomik `UPDATE ... WHERE status=...`.

## Katmanlı yapı (backend)

```
cmd/api        HTTP sunucu entrypoint
cmd/worker     arka plan görevleri (hold/sipariş süresi dolumu)
cmd/migrate    şema yönetimi (golang-migrate, gömülü SQL)
cmd/seed       örnek veri
internal/
  config       ortam değişkenlerinden ayar
  domain       iş nesneleri (Event, Order, User, ...) — katmanların ortak dili
  repository   SQL (pgx) — tek DB katmanı
  service      iş kuralları (fiyat, yetki, idempotency, telafi edici işlem)
  server       HTTP handler'lar + middleware (auth, CORS, request-id, recover, log)
  auth         bcrypt şifre hash + JWT (HS256)
  lock         Redis dağıtık kilit (SET NX PX + Lua compare-and-delete)
web/           React + Vite + TypeScript SPA
```

## Çalıştırma

### 1. Altyapı
```bash
docker compose up -d          # Postgres :5433 + Redis :6379
cp .env.example .env
```

### 2. Şema + örnek veri
```bash
go run ./cmd/migrate up
go run ./cmd/seed
```

### 3. Servisler (3 ayrı terminal)
```bash
go run ./cmd/api              # http://localhost:8080
go run ./cmd/worker           # arka plan temizlik
cd web && npm install && npm run dev   # http://localhost:5173
```

### Ya da hepsi konteynerde
```bash
docker compose --profile full up --build
# API :8080, frontend :8081
```

## Testler

```bash
go test ./... -race                 # birim testleri (auth, dağıtık kilit)
python scripts/e2e.py               # uçtan uca akış (API ayakta olmalı)
```

`e2e.py` şunları doğrular: kayıt/giriş, auth guard'ları, hold/release, checkout,
idempotency, mock ödeme (başarılı + reddedilen), sipariş durum geçişleri,
**overselling savunması** ve **25 eşzamanlı alıcı → tam 1 kazanan**.

## Önemli akışlar

| Akış | Ne oluyor |
|---|---|
| **Hold** | Redis `SET NX PX 10dk` → başarılıysa Postgres `available→held` (held_by=user). Postgres başarısızsa Redis kilidi geri alınır (telafi edici işlem). |
| **Checkout** | `pending` sipariş; her koltuk `FOR UPDATE` ile kilitlenip "hâlâ bende held mi" doğrulanır; fiyat sipariş anında sabitlenir; hold süresi 15 dk'ya uzatılır. `Idempotency-Key` header'ı çift-tıklamayı engeller. |
| **Ödeme (mock)** | Tek transaction: ödeme kaydı + her koltuk `held→sold` (koşullu) + sipariş `paid`. Bir koltuk kaybedildiyse tüm işlem geri sarılır. Kart son hanesi tek → reddedilir (test kolaylığı). |
| **Süre dolumu** | Worker 15 sn'de bir: `pending & expires_at<now` siparişleri `expired` yapar, koltukları bırakır; sonra siparişe bağlı olmayan süresi geçmiş hold'ları bırakır. |
