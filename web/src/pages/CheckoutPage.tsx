import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, ApiError, formatPrice } from "../api";
import { useCart } from "../cart";
import { Countdown } from "../components/Countdown";
import type { OrderDetail, Payment } from "../types";

export function CheckoutPage() {
  const cart = useCart();
  const nav = useNavigate();
  const [card, setCard] = useState("4242 4242 4242 4242");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Cift tiklama / retry guvenligi: sabit bir idempotency anahtari.
  const idemKey = useMemo(() => crypto.randomUUID(), []);

  const seats = cart.seats;
  if (seats.length === 0) {
    return (
      <div className="narrow">
        <h1>Sepet</h1>
        <p className="muted">Sepetin bos. <Link to="/">Etkinliklere bak</Link>.</p>
      </div>
    );
  }

  async function pay() {
    setErr(null);
    setBusy(true);
    try {
      const order = await api<OrderDetail>("/orders", {
        method: "POST",
        headers: { "Idempotency-Key": idemKey },
        body: { seats: seats.map((s) => ({ event_id: s.eventId, seat_id: s.seatId })) },
      });
      const payment = await api<Payment>(`/orders/${order.id}/payment`, {
        method: "POST",
        body: { card_number: card.replace(/\s/g, "") },
      });
      if (payment.status === "succeeded") {
        cart.clear();
        nav(`/orders/${order.id}`);
      } else {
        setErr("Odeme reddedildi. Farkli bir kart deneyin (cift haneyle biten kartlar onaylanir).");
      }
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="narrow">
      <h1>Sepet & Odeme</h1>
      <table className="table">
        <thead>
          <tr>
            <th>Koltuk</th>
            <th>Etkinlik</th>
            <th>Sure</th>
            <th className="right">Fiyat</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {seats.map((s) => (
            <tr key={s.seatId}>
              <td>{s.section}{s.row}-{s.number}</td>
              <td>{s.eventTitle}</td>
              <td><Countdown expiresAt={s.expiresAt} onExpire={() => cart.remove(s.seatId)} /></td>
              <td className="right">{formatPrice(s.priceCents, s.currency)}</td>
              <td>
                <button className="linkbtn" onClick={() => cart.remove(s.seatId)}>
                  cikar
                </button>
              </td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr>
            <td colSpan={3}><b>Toplam</b></td>
            <td className="right"><b>{formatPrice(cart.totalCents)}</b></td>
            <td></td>
          </tr>
        </tfoot>
      </table>

      <label className="form">
        Kart numarasi <span className="muted">(mock: cift haneyle biten onaylanir, tek reddedilir)</span>
        <input value={card} onChange={(e) => setCard(e.target.value)} />
      </label>

      {err && <p className="error">{err}</p>}
      <button className="btn" disabled={busy} onClick={pay}>
        {busy ? "Odeme yapiliyor..." : `${formatPrice(cart.totalCents)} ode`}
      </button>
    </div>
  );
}
