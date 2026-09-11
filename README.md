# ticketsale — koltuklu bilet satış uygulaması

Go + React + PostgreSQL + Redis ile yazılmış, koltuk seçimli (reserved seating) bilet
satış uygulaması. Bilet satışındaki backend / güvenlik / veritabanı incelikleri
(race condition, overselling, dağıtık kilit, transaction, idempotency, JWT) üzerine
kurulu bir öğrenme projesi.

## Mimari

```
[React SPA :5173] ──HTTP/JSON──▶ [Go API :8080] ──▶ [PostgreSQL :5433]  kalıcı gerçek: users,
      │                             │                                   events, seats, orders...
      └── JWT (localStorage)        ├──▶ [Redis :6379]   geçici: koltuk hold kilitleri
                                     └──▶ [Loki :3100] ──▶ [Grafana :3000]
                                          iş olayları:      dashboard +
[Go worker] ─────────────────────────────▶ hold/satın alma/bırakma   canlı log arama
   süresi dolan hold/siparişleri temizler, olayları da loglar
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
  logging      slog.Handler'lar: stdout + Grafana Loki'ye push (MultiHandler, LokiHandler)
  eventlog     iş olayı logları (seat_held / seat_purchased / seat_released / ...)
web/           React + Vite + TypeScript SPA
deploy/
  loki         Loki'nin minimal dev konfigürasyonu
  grafana      Grafana'nın otomatik yüklenen datasource + dashboard'u
```

## Çalıştırma

### 1. Altyapı
```bash
docker compose up -d          # Postgres :5433 + Redis :6379 + Loki :3100 + Grafana :3000
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

## Loglama & Grafana

API ve worker, her iş olayında (`internal/eventlog`) hem stdout'a hem **Grafana Loki**'ye
(`internal/logging`) yapısal bir log yazar. Loglama **asenkron ve non-blocking**'tir —
Loki ayakta olmasa bile uygulama normal çalışmaya devam eder, sadece o loglar Grafana'da
görünmez.

```bash
docker compose up -d          # loki + grafana da bu setin içinde
open http://localhost:3000    # admin / admin (ilk girişte "Skip" diyebilirsin)
```

**"TicketSale" klasöründeki "TicketSale - Bilet Olayları" dashboard'u otomatik yüklenir** —
elle datasource/dashboard eklemeye gerek yok (`deploy/grafana/provisioning`). Panellerde:
tutulan/satılan/bırakılan koltuk sayacı, süresi dolan sipariş + reddedilen ödeme sayacı,
olay türüne göre zaman serisi grafiği, ve canlı, okunabilir log akışı.

Loglanan olaylar: `seat_held`, `seat_purchased`, `seat_released` (`reason`:
`user_cancelled` | `hold_expired` | `order_expired`), `order_expired`, `payment_failed`.

Kendi LogQL sorgunu Grafana **Explore**'da (sol menü) yazmak istersen:
```logql
# Belirli bir kullanıcının tüm bilet hareketleri
{service=~"ticketsale-.*"} | json | user_id="<uuid>"

# Sadece satın almalar, okunabilir satır olarak
{event="seat_purchased"} | json | line_format "{{.msg}} — {{.amount_cents}} {{.currency}}"

# Son 1 saatte kaç bilet satıldı
sum(count_over_time({event="seat_purchased"}[1h]))
```

> **Kardinalite notu:** Loki'de sadece `service` ve `event` etiket (label) olarak indekslenir
> — ikisi de az sayıda sabit değer alır. `seat_id`/`user_id`/`order_id` gibi neredeyse sonsuz
> farklı değer alabilen alanlar etiket YAPILMAZ (Loki'nin index'i patlar); bunlar log
> satırının JSON gövdesinde kalır ve `| json` ile sorgulanır. Bkz. `internal/logging/loki.go`.
