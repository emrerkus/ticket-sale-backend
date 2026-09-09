import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, formatPrice } from "../api";
import type { Order } from "../types";

const STATUS_TR: Record<string, string> = {
  pending: "odeme bekleniyor",
  paid: "odendi",
  cancelled: "iptal",
  expired: "suresi doldu",
};

export function OrdersPage() {
  const [orders, setOrders] = useState<Order[] | null>(null);

  useEffect(() => {
    api<{ orders: Order[] }>("/orders").then((d) => setOrders(d.orders ?? []));
  }, []);

  if (!orders) return <p className="muted">Yukleniyor...</p>;
  if (orders.length === 0)
    return <p className="muted">Henuz siparisin yok. <Link to="/">Etkinliklere bak</Link>.</p>;

  return (
    <>
      <h1>Siparislerim</h1>
      <table className="table">
        <thead>
          <tr><th>Tarih</th><th>Durum</th><th className="right">Tutar</th><th></th></tr>
        </thead>
        <tbody>
          {orders.map((o) => (
            <tr key={o.id}>
              <td>{new Date(o.created_at).toLocaleString("tr-TR")}</td>
              <td><span className={`pill ${o.status === "paid" ? "ok" : ""}`}>{STATUS_TR[o.status]}</span></td>
              <td className="right">{formatPrice(o.total_cents, o.currency)}</td>
              <td><Link to={`/orders/${o.id}`}>detay</Link></td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
