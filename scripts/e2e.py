#!/usr/bin/env python3
"""Uçtan uca akış testi: register -> login -> hold -> checkout -> pay.

Kullanım:
    python scripts/e2e.py            # API http://localhost:8080'de calisiyor olmali
    BASE=http://localhost:8080 python scripts/e2e.py

Once altyapiyi ayaga kaldir ve seed'le:
    docker compose up -d
    go run ./cmd/migrate up
    go run ./cmd/seed
    go run ./cmd/api
"""
import json
import os
import sys
import threading
import time
import urllib.error
import urllib.request

BASE = os.environ.get("BASE", "http://localhost:8080")
FAILS = 0


def call(method, path, body=None, token=None, headers=None):
    url = BASE + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req) as r:
            raw = r.read().decode()
            return r.status, (json.loads(raw) if raw else None)
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        return e.code, (json.loads(raw) if raw else None)


def check(name, got, want):
    global FAILS
    ok = got == want
    mark = "OK  " if ok else "FAIL"
    print(f"  [{mark}] {name}: beklenen {want}, gelen {got}")
    if not ok:
        FAILS += 1


def main():
    email = f"e2e_{int(time.time())}@example.com"
    pw = "supersecret1"

    print("1) Katalog")
    st, events = call("GET", "/events")
    check("GET /events -> 200", st, 200)
    assert events["events"], "seed calistirilmamis: yayinda etkinlik yok"
    ev = events["events"][0]
    eid = ev["id"]

    st, detail = call("GET", f"/events/{eid}")
    check("GET /events/{id} -> 200", st, 200)
    free = [s for s in detail["seat_map"] if s["status"] == "available"][:2]
    assert len(free) == 2, "yeterli bos koltuk yok"
    s1, s2 = free[0]["seat_id"], free[1]["seat_id"]

    print("2) Kimlik")
    st, reg = call("POST", "/auth/register", {"email": email, "password": pw})
    check("register -> 201", st, 201)
    token = reg["token"]
    st, _ = call("POST", "/auth/login", {"email": email, "password": pw})
    check("login -> 200", st, 200)
    st, _ = call("POST", "/auth/login", {"email": email, "password": "yanlis"})
    check("yanlis sifre -> 401", st, 401)
    st, _ = call("POST", f"/events/{eid}/seats/{s1}/hold")
    check("token'siz hold -> 401", st, 401)

    print("3) Hold")
    st, h1 = call("POST", f"/events/{eid}/seats/{s1}/hold", {}, token)
    check("hold seat1 -> 200", st, 200)
    st, _ = call("POST", f"/events/{eid}/seats/{s2}/hold", {}, token)
    check("hold seat2 -> 200", st, 200)
    st, _ = call("POST", f"/events/{eid}/seats/{s1}/hold", {}, token)
    check("ayni koltugu tekrar hold (ayni kullanici, kilit hala bende) -> 409", st, 409)

    print("4) Checkout")
    seats = [{"event_id": eid, "seat_id": s1}, {"event_id": eid, "seat_id": s2}]
    st, order = call("POST", "/orders", {"seats": seats}, token, {"Idempotency-Key": "e2e-1"})
    check("checkout -> 201", st, 201)
    oid = order["id"]
    check("toplam = 2 x koltuk fiyati", order["total_cents"], sum(i["unit_price_cents"] for i in order["items"]))
    st, order2 = call("POST", "/orders", {"seats": seats[:1]}, token, {"Idempotency-Key": "e2e-1"})
    check("ayni idempotency key -> ayni siparis", order2["id"], oid)

    print("5) Odeme")
    st, pay_fail = call("POST", f"/orders/{oid}/payment", {"card_number": "4111111111111113"}, token)
    check("tek haneyle biten kart -> 402 (reddedildi)", st, 402)
    check("odeme durumu failed", pay_fail["status"], "failed")

    st, pay_ok = call("POST", f"/orders/{oid}/payment", {"card_number": "4242424242424242"}, token)
    check("cift haneyle biten kart -> 200 (basarili)", st, 200)
    check("odeme durumu succeeded", pay_ok["status"], "succeeded")

    st, final = call("GET", f"/orders/{oid}", None, token)
    check("siparis paid oldu", final["status"], "paid")

    st, dup = call("POST", f"/orders/{oid}/payment", {"card_number": "4242424242424242"}, token)
    check("odenen siparise tekrar odeme -> 409", st, 409)

    print("6) Overselling savunmasi")
    # Baska kullanici, ayni (artik sold) koltugu hold etmeye calisir.
    st, r2 = call("POST", "/auth/register", {"email": f"b_{email}", "password": pw})
    t2 = r2["token"]
    st, _ = call("POST", f"/events/{eid}/seats/{s1}/hold", {}, t2)
    check("sold koltugu hold -> 409", st, 409)

    print("7) Eszamanlilik: 25 alici, TEK koltuk")
    st, detail = call("GET", f"/events/{eid}")
    target = next(s["seat_id"] for s in detail["seat_map"] if s["status"] == "available")
    tokens = []
    for i in range(25):
        _, r = call("POST", "/auth/register", {"email": f"rush_{time.time()}_{i}@x.com", "password": pw})
        tokens.append(r["token"])

    wins = []
    lock = threading.Lock()

    def buy(tok):
        s, _ = call("POST", f"/events/{eid}/seats/{target}/hold", {}, tok)
        if s != 200:
            return
        s, o = call("POST", "/orders", {"seats": [{"event_id": eid, "seat_id": target}]}, tok)
        if s != 201:
            return
        s, p = call("POST", f"/orders/{o['id']}/payment", {"card_number": "4242424242424242"}, tok)
        if s == 200 and p["status"] == "succeeded":
            with lock:
                wins.append(tok)

    ths = [threading.Thread(target=buy, args=(t,)) for t in tokens]
    for t in ths:
        t.start()
    for t in ths:
        t.join()
    check("25 aliciden TAM 1'i koltugu satin aldi", len(wins), 1)

    print()
    if FAILS:
        print(f"SONUC: {FAILS} test BASARISIZ")
        sys.exit(1)
    print("SONUC: tum testler gecti")


if __name__ == "__main__":
    main()
