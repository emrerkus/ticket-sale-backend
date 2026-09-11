// Package logging log/slog icin ek handler'lar saglar: ayni log kaydini birden
// fazla hedefe (stdout + Loki) yollayan bir MultiHandler, ve Grafana Loki'ye
// HTTP ile log basan bir LokiHandler.
package logging

import (
	"context"
	"log/slog"
)

// MultiHandler tek bir slog.Record'u BIRDEN FAZLA handler'a dagitir.
// log/slog'un standart kutuphanesinde bu yok; kendimiz yaziyoruz.
//
// Neden gerekli? "Terminalde de gorunsun (docker logs / go run ciktisi),
// Grafana'da da aranabilsin" istiyoruz -- ayni satiri iki yere yazmak.
type MultiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

// Enabled: handler'lardan HERHANGI BIRI bu seviyede ilgileniyorsa true.
func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle: kaydi TUM handler'lara yollar. Biri hata verirse digerlerini
// engellemez -- bir logun kaybolmasi digerinin de kaybolmasina sebep olmamali.
func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: next}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: next}
}
