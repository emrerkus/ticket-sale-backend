package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/emrerkus/ticket-sale-backend/internal/domain"
	"github.com/emrerkus/ticket-sale-backend/internal/service"
)

type checkoutRequest struct {
	Seats []domain.SeatRef `json:"seats"`
}

type payRequest struct {
	// Mock: card_number son hanesi TEK ise odeme reddedilir (test kolayligi).
	CardNumber string `json:"card_number"`
	Fail       bool   `json:"fail"`
}

// POST /orders  (authed)
func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz istek govdesi"})
		return
	}
	userID := UserID(r.Context())
	idemKey := r.Header.Get("Idempotency-Key")

	order, err := s.orderSvc.Checkout(r.Context(), userID, req.Seats, idemKey)
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, order)
	case errors.Is(err, service.ErrInvalidID):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz id"})
	case errors.Is(err, service.ErrEmptyCart):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sepet bos"})
	case errors.Is(err, service.ErrCartTooLarge):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tek sipariste en fazla 10 koltuk"})
	case errors.Is(err, service.ErrMixedCurrency):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "koltuklar farkli para birimlerinde"})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "koltuk bulunamadi"})
	case errors.Is(err, service.ErrSeatNotHeld):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "koltuklardan biri sende tutulmuyor veya baska bir sipariste"})
	default:
		s.log.Error("checkout hatasi", "user_id", userID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
	}
}

// GET /orders  (authed)
func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.orderSvc.List(r.Context(), UserID(r.Context()))
	if err != nil {
		s.log.Error("siparis listesi hatasi", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

// GET /orders/{id}  (authed)
func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	order, err := s.orderSvc.Get(r.Context(), UserID(r.Context()), r.PathValue("id"))
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, order)
	case errors.Is(err, service.ErrInvalidID):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz id"})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "siparis bulunamadi"})
	default:
		s.log.Error("siparis detay hatasi", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
	}
}

// POST /orders/{id}/payment  (authed)
func (s *Server) handlePayOrder(w http.ResponseWriter, r *http.Request) {
	var req payRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz istek govdesi"})
		return
	}
	// Mock kural: kart no TEK haneyle bitiyorsa (veya fail=true) odeme reddedilir.
	fail := req.Fail
	if n := len(req.CardNumber); n > 0 {
		last := req.CardNumber[n-1]
		if last >= '0' && last <= '9' && (last-'0')%2 == 1 {
			fail = true
		}
	}

	pay, err := s.orderSvc.Pay(r.Context(), UserID(r.Context()), r.PathValue("id"), fail)
	switch {
	case err == nil && pay.Status == "succeeded":
		writeJSON(w, http.StatusOK, pay)
	case err == nil: // failed payment -> 402 Payment Required
		writeJSON(w, http.StatusPaymentRequired, pay)
	case errors.Is(err, service.ErrInvalidID):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz id"})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "siparis bulunamadi"})
	case errors.Is(err, service.ErrOrderNotPayable):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "siparis odenebilir durumda degil (odendi, iptal edildi veya suresi doldu)"})
	case errors.Is(err, service.ErrSeatLost):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "koltuklardan biri artik uygun degil, siparis tamamlanamadi"})
	default:
		s.log.Error("odeme hatasi", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
	}
}
