package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/emrerkus/ticket-sale-backend/internal/auth"
)

// ctxKey: context'e deger koyarken cakismayi onlemek icin ozel tip.
type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyUserID
)

// chain birden cok middleware'i tek bir handler'a sarar (ilk verilen en dista).
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// recoverer: bir handler panik ederse tum sunucu cokmesin; 500 donup loglar.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panik yakalandi", "err", rec, "path", r.URL.Path,
					"request_id", RequestID(r.Context()))
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// requestID: her istege bir kimlik verir (loglarda takip icin) ve yanit header'ina koyar.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestLogger: her istegi metot/path/status/sure ile loglar.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Info("istek",
			"method", r.Method, "path", r.URL.Path, "status", sw.status,
			"dur_ms", time.Since(start).Milliseconds(), "request_id", RequestID(r.Context()))
	})
}

// cors: frontend baska bir origin'de (localhost:5173) oldugu icin gerekli.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.CORSOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authed: bir handler'i "gecerli JWT gerekli" hale getirir. Token'daki kullanici
// id'sini context'e koyar; handler UserID(ctx) ile alir.
func (s *Server) authed(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(raw, "Bearer ")
		if !ok || token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "giris yapmalisin"})
			return
		}
		userID, err := s.tokens.Parse(token)
		if err != nil {
			if err == auth.ErrInvalidToken {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "gecersiz veya suresi dolmus token"})
				return
			}
			s.log.Error("token parse hatasi", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sunucu hatasi"})
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
		h(w, r.WithContext(ctx))
	}
}

// UserID: authed middleware'inden gecmis bir istekte kullanici id'sini doner.
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUserID).(string)
	return v
}

// RequestID: mevcut istegin kimligini doner.
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// statusWriter: yazilan HTTP status kodunu yakalar (loglama icin).
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
