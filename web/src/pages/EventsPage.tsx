import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import type { EventListItem } from "../types";

export function EventsPage() {
  const [events, setEvents] = useState<EventListItem[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    api<{ events: EventListItem[] }>("/events")
      .then((d) => setEvents(d.events ?? []))
      .catch((e) => setErr(String(e.message)));
  }, []);

  if (err) return <p className="error">Etkinlikler yuklenemedi: {err}</p>;
  if (!events) return <p className="muted">Yukleniyor...</p>;
  if (events.length === 0) return <p className="muted">Yayinda etkinlik yok.</p>;

  return (
    <>
      <h1>Etkinlikler</h1>
      <div className="cards">
        {events.map((e) => (
          <Link key={e.id} to={`/events/${e.id}`} className="card">
            <h3>{e.title}</h3>
            <p className="muted">{e.venue_name}</p>
            <p>{new Date(e.starts_at).toLocaleString("tr-TR")}</p>
            <p className="desc">{e.description}</p>
          </Link>
        ))}
      </div>
    </>
  );
}
