import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, formatPrice } from "../api";
import type { OrderDetail } from "../types";

const STATUS_TR: Record<string, string> = {
  pending: "odeme bekleniyor",
  paid: "odendi",
  cancelled: "iptal edildi",
  expired: "suresi doldu",
};

export function OrderPage() {
  const { id } = useParams<{ id: string }>();
  const [order, setOrder] = useState<OrderDetail | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    api<OrderDetail>(`/orders/${id}`).then(setOrder).catch((e) => setErr(String(e.message)));
  }, [id]);

  if (err) return <p className="error">{err}</p>;
  if (!order) return <p className="muted">Yukleniyor...</p>;

  return (
    <div className="narrow">
      {order.status === "paid" && <h1>🎉 Biletlerin hazir</h1>}
      {order.status !== "paid" && <h1>Siparis</h1>}

      <p>
        Durum: <span className={`pill ${order.status === "paid" ? "ok" : ""}`}>
          {STATUS_TR[order.status] ?? order.status}
        </span>
      </p>

      <table className="table">
        <thead>
          <tr><th>Koltuk</th><th className="right">Fiyat</th></tr>
        </thead>
        <tbody>
          {order.items.map((it) => (
            <tr key={it.id}>
              <td>{it.section}{it.row_label}-{it.seat_number}</td>
              <td className="right">{formatPrice(it.unit_price_cents, order.currency)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr>
            <td><b>Toplam</b></td>
            <td className="right"><b>{formatPrice(order.total_cents, order.currency)}</b></td>
          </tr>
        </tfoot>
      </table>

      {order.payment && (
        <p className="muted">
          Odeme: {order.payment.status} · {order.payment.provider} · {order.payment.provider_ref}
        </p>
      )}

      <p><Link to="/siparislerim">Tum siparislerim</Link> · <Link to="/">Etkinlikler</Link></p>
    </div>
  );
}
