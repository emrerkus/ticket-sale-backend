package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// handleHealthz — LIVENESS. "Surec ayakta ve cevap verebiliyor mu?"
// Hicbir bagimliligi kontrol ETMEZ. Basarisiz olursa orchestrator konteyneri
// oldurup yeniden baslatir; o yuzden DB'ye vs. bakmamali.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz — READINESS. "Trafik almaya HAZIR miyim?"
// Tum kritik bagimliliklari (Postgres, Redis) yoklar. Biri bile calismiyorsa
// 503 doner -> load balancer bu instance'a istek yollamayi keser (ama oldurmez).
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	// DB/Redis yanit vermiyorsa istegi sonsuz bekletip goroutine tuketmeyelim.
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := make(map[string]string)
	overallStatus := "ok"
	httpStatus := http.StatusOK

	if err := s.db.Ping(ctx); err != nil {
		checks["postgres"] = "error: " + err.Error()
		overallStatus = "unavailable"
		httpStatus = http.StatusServiceUnavailable
	} else {
		checks["postgres"] = "ok"
	}

	if err := s.rdb.Ping(ctx).Err(); err != nil {
		checks["redis"] = "error: " + err.Error()
		overallStatus = "unavailable"
		httpStatus = http.StatusServiceUnavailable
	} else {
		checks["redis"] = "ok"
	}

	writeJSON(w, httpStatus, map[string]any{
		"status": overallStatus,
		"checks": checks,
	})
}

// writeJSON: bir degeri JSON'a cevirip yazar. ONCE bir buffer'a encode eder;
// boylece encode YARIDA hata verirse henuz hicbir sey yazilmamis olur ve
// dogru bir 500 donebiliriz (status + govde tutarli kalir).
func writeJSON(w http.ResponseWriter, status int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"status":"error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf)
}
