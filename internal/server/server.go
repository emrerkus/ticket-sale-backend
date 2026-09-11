// Package server HTTP sunucusunu kurar ve yasam dongusunu (baslat / duzgun kapat)
// yonetir. Rotalar routes.go'da, handler'lar ayri dosyalarda, ara katmanlar middleware.go'da.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/emrerkus/ticket-sale-backend/internal/auth"
	"github.com/emrerkus/ticket-sale-backend/internal/config"
	"github.com/emrerkus/ticket-sale-backend/internal/eventlog"
	"github.com/emrerkus/ticket-sale-backend/internal/lock"
	"github.com/emrerkus/ticket-sale-backend/internal/repository"
	"github.com/emrerkus/ticket-sale-backend/internal/service"
)

// Server uygulamanin HTTP katmanini temsil eder. Bagimliliklar burada tutulur;
// handler'lar dogrudan SQL/repository gormez, sadece servislerle konusur.
type Server struct {
	cfg     config.Config
	log     *slog.Logger
	db      *pgxpool.Pool
	rdb     *redis.Client
	httpSrv *http.Server

	tokens *auth.TokenManager

	authSvc  *service.AuthService
	eventSvc *service.EventService
	holdSvc  *service.SeatHoldService
	orderSvc *service.OrderService
}

// New bir Server kurar ama HENUZ dinlemeye baslamaz.
// events: is olaylarini (hold/satin alma/birakma) Grafana Loki'ye ve stdout'a
// yazan logger (bkz. internal/eventlog, cmd/api/main.go'da kuruluyor).
func New(cfg config.Config, log *slog.Logger, db *pgxpool.Pool, rdb *redis.Client, events *eventlog.Logger) *Server {
	// Katman zinciri: pgxpool -> repository -> service -> handler.
	userRepo := repository.NewUserRepository(db)
	eventRepo := repository.NewEventRepository(db)
	orderRepo := repository.NewOrderRepository(db)

	locker := lock.New(rdb)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)

	s := &Server{
		cfg: cfg, log: log, db: db, rdb: rdb, tokens: tokens,
		authSvc:  service.NewAuthService(userRepo, tokens),
		eventSvc: service.NewEventService(eventRepo),
		holdSvc:  service.NewSeatHoldService(eventRepo, locker, events),
		orderSvc: service.NewOrderService(orderRepo, events),
	}

	// Global middleware zinciri (disdan ice): recoverer -> requestID -> logger -> cors.
	handler := chain(s.routes(),
		s.recoverer,
		requestID,
		s.requestLogger,
		s.cors,
	)

	s.httpSrv = &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: handler,

		// Timeout'lar: yavas/kotu niyetli istemcinin baglantiyi sonsuz tutmasini
		// engeller (Slowloris). Go'nun varsayilani sinirsizdir.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return s
}

// Run sunucuyu baslatir ve ctx iptal edilene kadar bloklar; sonra graceful shutdown yapar.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("HTTP sunucu dinliyor", "addr", s.httpSrv.Addr, "env", s.cfg.AppEnv)
		err := s.httpSrv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("kapanma sinyali alindi, graceful shutdown basliyor", "timeout", s.cfg.ShutdownTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			s.log.Error("graceful shutdown basarisiz, zorla kapatiliyor", "err", err)
			_ = s.httpSrv.Close()
			return err
		}
		s.log.Info("sunucu temiz kapandi")
		return nil
	}
}
