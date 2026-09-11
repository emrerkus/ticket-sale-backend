package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/emrerkus/ticket-sale-backend/internal/domain"
	"github.com/emrerkus/ticket-sale-backend/internal/eventlog"
	"github.com/emrerkus/ticket-sale-backend/internal/lock"
	"github.com/emrerkus/ticket-sale-backend/internal/repository"
)

// HoldDuration: bir koltuk sepete alindiginda ne kadar rezerve kalir.
// Suresi gecen hold'lari worker (cmd/worker) serbest birakir.
const HoldDuration = 10 * time.Minute

// ErrSeatUnavailable: koltuk var ama su an alinamaz (zaten held/sold).
var ErrSeatUnavailable = errors.New("service: koltuk musait degil")

// SeatHoldService, koltuk tutma islemini YONETIR: once Redis'te hizli bir
// kilit alir (kullanici deneyimi icin), sonra Postgres'i o kilitle TUTARLI
// hale getirir (gercek dogruluk kaynagi). Ikisi arasinda uyusmazlik cikarsa
// (ornegin Postgres basarisiz olursa) Redis kilidini geri alir.
type SeatHoldService struct {
	repo   *repository.EventRepository
	locker *lock.Locker
	events *eventlog.Logger
}

func NewSeatHoldService(repo *repository.EventRepository, locker *lock.Locker, events *eventlog.Logger) *SeatHoldService {
	return &SeatHoldService{repo: repo, locker: locker, events: events}
}

// lockKey: Redis'teki kilit anahtari. event_id+seat_id kombinasyonu,
// event_seats tablosundaki UNIQUE(event_id, seat_id) kisitiyla birebir eslesir
// -- yani bu key, tam olarak "bir envanter satiri" demek.
func lockKey(eventID, seatID string) string {
	return "hold:" + eventID + ":" + seatID
}

// HoldSeat bir koltugu HoldDuration suresince gecici olarak rezerve eder.
//
// Iki asamali, Redis + Postgres'i birlikte tutarli tutan islem:
//  1. Redis kilidini al (SET NX). Alamazsak -> baskasi tutuyor -> ErrSeatUnavailable.
//  2. Postgres'te koltugu 'held' yap (held_by = holderID).
//  3. Postgres basarisizsa Redis kilidini GERI AL (telafi edici islem) --
//     yoksa Redis "held" derken Postgres "available" der ve kilit suresi
//     dolana kadar (10 dk) koltuk kimseye gorunmez.
func (s *SeatHoldService) HoldSeat(ctx context.Context, eventID, seatID, holderID string) (*domain.SeatHold, error) {
	if _, err := uuid.Parse(eventID); err != nil {
		return nil, ErrInvalidID
	}
	if _, err := uuid.Parse(seatID); err != nil {
		return nil, ErrInvalidID
	}
	if _, err := uuid.Parse(holderID); err != nil {
		return nil, ErrInvalidID
	}

	key := lockKey(eventID, seatID)
	token, ok, err := s.locker.Acquire(ctx, key, HoldDuration)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrSeatUnavailable
	}

	heldUntil := time.Now().Add(HoldDuration)
	updated, exists, err := s.repo.TryHold(ctx, eventID, seatID, holderID, heldUntil)

	if err != nil || !updated {
		_ = s.locker.Release(ctx, key, token) // best-effort telafi
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNotFound
		}
		return nil, ErrSeatUnavailable
	}

	s.events.SeatHeld(ctx, eventID, seatID, holderID, heldUntil)

	return &domain.SeatHold{
		EventID:   eventID,
		SeatID:    seatID,
		HolderID:  holderID,
		Status:    "held",
		ExpiresAt: heldUntil,
	}, nil
}

// ReleaseSeat bir hold'u iptal eder. SADECE hold'un sahibi (holderID) iptal
// edebilir -- bu kontrol Postgres tarafinda (held_by = holderID) yapilir.
// Redis kilidi ForceRelease ile silinir (token elimizde yok, ama sahiplik
// Postgres eslesmesiyle zaten kanitlandi).
func (s *SeatHoldService) ReleaseSeat(ctx context.Context, eventID, seatID, holderID string) error {
	for _, id := range []string{eventID, seatID, holderID} {
		if _, err := uuid.Parse(id); err != nil {
			return ErrInvalidID
		}
	}

	released, err := s.repo.ReleaseHold(ctx, eventID, seatID, holderID)
	if err != nil {
		return err
	}
	if !released {
		return ErrNotFound
	}

	// Postgres sahipligi dogruladi -> Redis kilidini de temizle (best-effort).
	_ = s.locker.ForceRelease(ctx, lockKey(eventID, seatID))
	s.events.SeatReleased(ctx, eventID, seatID, holderID, eventlog.ReasonUser)
	return nil
}
