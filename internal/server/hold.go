package server

import (
	"errors"
	"net/http"

	"github.com/emrerkus/ticket-sale-backend/internal/service"
)

// handleHoldSeat — POST /events/{eventID}/seats/{seatID}/hold  (authed)
// Koltugu 10 dakikaligina, GIRIS YAPMIS kullanici adina tutar.
func (s *Server) handleHoldSeat(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("eventID")
	seatID := r.PathValue("seatID")
	userID := UserID(r.Context()) // JWT'den gelir, istemci soyleyemez

	hold, err := s.holdSvc.HoldSeat(r.Context(), eventID, seatID, userID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, hold)
	case errors.Is(err, service.ErrInvalidID):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz id"})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "koltuk bulunamadi"})
	case errors.Is(err, service.ErrSeatUnavailable):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "koltuk su anda musait degil"})
	default:
		s.log.Error("koltuk tutulamadi", "event_id", eventID, "seat_id", seatID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
	}
}

// handleReleaseSeat — DELETE /events/{eventID}/seats/{seatID}/hold  (authed)
// Bir hold'u iptal eder. Sadece hold'un sahibi iptal edebilir (userID = held_by).
func (s *Server) handleReleaseSeat(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("eventID")
	seatID := r.PathValue("seatID")
	userID := UserID(r.Context())

	err := s.holdSvc.ReleaseSeat(r.Context(), eventID, seatID, userID)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, service.ErrInvalidID):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz id"})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "iptal edilecek hold bulunamadi"})
	default:
		s.log.Error("hold iptal edilemedi", "event_id", eventID, "seat_id", seatID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
	}
}
