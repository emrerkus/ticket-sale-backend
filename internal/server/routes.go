package server

import "net/http"

// routes tum HTTP rotalarini tek bir yerde toplar.
// Go 1.22+ ServeMux "METHOD /path" ve "/path/{param}" kaliplarini destekler.
// s.authed(...) ile sarilan rotalar gecerli bir JWT ister.
func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	// --- Saglik ---
	mux.HandleFunc("GET /healthz", s.handleHealthz) // liveness
	mux.HandleFunc("GET /readyz", s.handleReadyz)   // readiness (DB + Redis)

	// --- Kimlik ---
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("GET /me", s.authed(s.handleMe))

	// --- Katalog (herkese acik) ---
	mux.HandleFunc("GET /events", s.handleListEvents)
	mux.HandleFunc("GET /events/{id}", s.handleGetEvent)

	// --- Koltuk hold'u (giris gerekli) ---
	mux.HandleFunc("POST /events/{eventID}/seats/{seatID}/hold", s.authed(s.handleHoldSeat))
	mux.HandleFunc("DELETE /events/{eventID}/seats/{seatID}/hold", s.authed(s.handleReleaseSeat))

	// --- Siparis & odeme (giris gerekli) ---
	mux.HandleFunc("POST /orders", s.authed(s.handleCheckout))
	mux.HandleFunc("GET /orders", s.authed(s.handleListOrders))
	mux.HandleFunc("GET /orders/{id}", s.authed(s.handleGetOrder))
	mux.HandleFunc("POST /orders/{id}/payment", s.authed(s.handlePayOrder))

	return mux
}
