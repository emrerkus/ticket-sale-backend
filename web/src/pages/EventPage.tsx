import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, ApiError, formatPrice } from "../api";
import { useAuth } from "../auth";
import { useCart } from "../cart";
import { SeatMap } from "../components/SeatMap";
import type { EventDetail, SeatHold, SeatMapEntry } from "../types";

export function EventPage() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const cart = useCart();
  const [ev, setEv] = useState<EventDetail | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const reload = useCallback(() => {
    if (!id) return;
    api<EventDetail>(`/events/${id}`)
      .then(setEv)
      .catch((e) => setErr(String(e.message)));
  }, [id]);

  useEffect(reload, [reload]);

  const priceOf = useMemo(() => {
    const map = new Map(ev?.ticket_types.map((t) => [t.id, t]) ?? []);
    return (seat: SeatMapEntry) => map.get(seat.ticket_type_id);
  }, [ev]);

  const selectedIds = useMemo(
    () => new Set(cart.seats.filter((s) => s.eventId === id).map((s) => s.seatId)),
    [cart.seats, id]
  );

  async function toggle(seat: SeatMapEntry) {
    if (!user || !ev) return;
    setNotice(null);
    setBusyId(seat.seat_id);
    try {
      if (selectedIds.has(seat.seat_id)) {
        await api(`/events/${ev.id}/seats/${seat.seat_id}/hold`, { method: "DELETE", body: {} });
        cart.remove(seat.seat_id);
      } else {
        const hold = await api<SeatHold>(
          `/events/${ev.id}/seats/${seat.seat_id}/hold`,
          { method: "POST", body: {} }
        );
        const tt = priceOf(seat);
        cart.add({
          eventId: ev.id,
          eventTitle: ev.title,
          seatId: seat.seat_id,
          section: seat.section,
          row: seat.row_label,
          number: seat.seat_number,
          priceCents: tt?.price_cents ?? 0,
          currency: tt?.currency ?? "TRY",
          expiresAt: hold.expires_at,
        });
      }
      reload();
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : String(e);
      setNotice(msg);
      reload();
    } finally {
      setBusyId(null);
    }
  }

  if (err) return <p className="error">{err}</p>;
  if (!ev) return <p className="muted">Yukleniyor...</p>;

  return (
    <>
      <p><Link to="/">&larr; Etkinlikler</Link></p>
      <h1>{ev.title}</h1>
      <p className="muted">
        {ev.venue.name} · {new Date(ev.starts_at).toLocaleString("tr-TR")}
      </p>
      <p>{ev.description}</p>

      <div className="prices">
        {ev.ticket_types.map((t) => (
          <span key={t.id} className="pill">
            {t.name}: {formatPrice(t.price_cents, t.currency)}
          </span>
        ))}
      </div>

      {!user && (
        <p className="banner">
          Koltuk secmek icin <Link to="/giris" state={{ from: `/events/${ev.id}` }}>giris yap</Link>.
        </p>
      )}
      {notice && <p className="error">{notice}</p>}

      <SeatMap
        seats={ev.seat_map}
        selectedIds={selectedIds}
        busyId={busyId}
        onToggle={toggle}
      />

      {selectedIds.size > 0 && (
        <div className="stickybar">
          {selectedIds.size} koltuk secildi ·{" "}
          {formatPrice(
            cart.seats.filter((s) => s.eventId === id).reduce((a, s) => a + s.priceCents, 0)
          )}
          <Link className="btn" to="/sepet">
            Sepete git
          </Link>
        </div>
      )}
    </>
  );
}
