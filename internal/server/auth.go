package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/emrerkus/ticket-sale-backend/internal/service"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// POST /auth/register
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz istek govdesi"})
		return
	}

	res, err := s.authSvc.Register(r.Context(), c.Email, c.Password)
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, res)
	case errors.Is(err, service.ErrWeakInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecerli bir email ve en az 8 karakter sifre gerekli"})
	case errors.Is(err, service.ErrEmailTaken):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "bu email zaten kayitli"})
	default:
		s.log.Error("register hatasi", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
	}
}

// POST /auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gecersiz istek govdesi"})
		return
	}

	res, err := s.authSvc.Login(r.Context(), c.Email, c.Password)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, res)
	case errors.Is(err, service.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "email veya sifre hatali"})
	default:
		s.log.Error("login hatasi", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
	}
}

// GET /me  (authed) — token gecerliyse "ben kimim" doner.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.authSvc.Me(r.Context(), UserID(r.Context()))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "oturum bulunamadi"})
		return
	}
	writeJSON(w, http.StatusOK, u)
}
