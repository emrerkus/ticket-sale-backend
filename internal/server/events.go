package server

import (
	"errors"
	"net/http"

	"github.com/emrerkus/ticket-sale-backend/internal/service"
)

// handleListEvents — GET /events
func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.eventSvc.List(r.Context())
	if err != nil {
		// Beklenmedik hatalari LOGLA ama istemciye ic detay SIZDIRMA.
		s.log.Error("etkinlik listesi alinamadi", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
		return
	}
	// events nil olabilir (hic yayinda etkinlik yoksa) -> JSON'da null; sorun degil.
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// handleGetEvent — GET /events/{id}
// Hata tipine gore HTTP kodu: gecersiz id -> 400, yok/taslak -> 404, digeri -> 500.
func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	detail, err := s.eventSvc.Get(r.Context(), id)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, detail)
	case errors.Is(err, service.ErrInvalidID):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz etkinlik id"})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "etkinlik bulunamadi"})
	default:
		s.log.Error("etkinlik detayi alinamadi", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
	}
}
