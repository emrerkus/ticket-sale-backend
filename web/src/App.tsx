import { Link, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useAuth } from "./auth";
import { useCart } from "./cart";
import { EventsPage } from "./pages/EventsPage";
import { EventPage } from "./pages/EventPage";
import { CheckoutPage } from "./pages/CheckoutPage";
import { OrderPage } from "./pages/OrderPage";
import { OrdersPage } from "./pages/OrdersPage";
import { AuthPage } from "./pages/AuthPage";

function RequireAuth({ children }: { children: JSX.Element }) {
  const { user, ready } = useAuth();
  const loc = useLocation();
  if (!ready) return <p className="muted">Yukleniyor...</p>;
  if (!user) return <Navigate to="/giris" state={{ from: loc.pathname }} replace />;
  return children;
}

export function App() {
  const { user, logout } = useAuth();
  const { seats } = useCart();

  return (
    <>
      <header className="topbar">
        <Link to="/" className="brand">
          🎟️ ticketsale
        </Link>
        <nav>
          <Link to="/sepet">Sepet ({seats.length})</Link>
          {user ? (
            <>
              <Link to="/siparislerim">Siparislerim</Link>
              <span className="muted">{user.email}</span>
              <button className="linkbtn" onClick={logout}>
                Cikis
              </button>
            </>
          ) : (
            <Link to="/giris">Giris</Link>
          )}
        </nav>
      </header>

      <main className="container">
        <Routes>
          <Route path="/" element={<EventsPage />} />
          <Route path="/events/:id" element={<EventPage />} />
          <Route path="/giris" element={<AuthPage />} />
          <Route
            path="/sepet"
            element={
              <RequireAuth>
                <CheckoutPage />
              </RequireAuth>
            }
          />
          <Route
            path="/siparislerim"
            element={
              <RequireAuth>
                <OrdersPage />
              </RequireAuth>
            }
          />
          <Route
            path="/orders/:id"
            element={
              <RequireAuth>
                <OrderPage />
              </RequireAuth>
            }
          />
          <Route path="*" element={<p>Sayfa bulunamadi.</p>} />
        </Routes>
      </main>
    </>
  );
}
