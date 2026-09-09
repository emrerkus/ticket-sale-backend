package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/emrerkus/ticket-sale-backend/internal/domain"
	"github.com/emrerkus/ticket-sale-backend/internal/repository"
)

// OrderTTL: bir 'pending' siparis odenmezse ne kadar sonra iptal (expired) olur.
// HoldDuration'dan (10dk) uzun olmali ki checkout ile odeme arasinda koltuk kacmasin.
const OrderTTL = 15 * time.Minute

// maxSeatsPerOrder: tek sipariste en fazla koltuk (kotu niyetli buyuk istekleri sinirlar).
const maxSeatsPerOrder = 10

var (
	ErrEmptyCart       = errors.New("service: sepet bos")
	ErrCartTooLarge    = errors.New("service: cok fazla koltuk")
	ErrSeatNotHeld     = errors.New("service: koltuk tutulmuyor veya baska bir sipariste")
	ErrOrderNotPayable = errors.New("service: siparis odenebilir durumda degil")
	ErrSeatLost        = errors.New("service: koltuk odeme sirasinda kaybedildi")
	ErrMixedCurrency   = errors.New("service: sepette birden fazla para birimi")
)

type OrderService struct {
	orders *repository.OrderRepository
}

func NewOrderService(orders *repository.OrderRepository) *OrderService {
	return &OrderService{orders: orders}
}

// Checkout tutulan koltuklardan bir 'pending' siparis olusturur.
func (s *OrderService) Checkout(ctx context.Context, userID string, seats []domain.SeatRef, idempotencyKey string) (*domain.OrderDetail, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidID
	}
	if len(seats) == 0 {
		return nil, ErrEmptyCart
	}
	if len(seats) > maxSeatsPerOrder {
		return nil, ErrCartTooLarge
	}
	for _, sr := range seats {
		if _, err := uuid.Parse(sr.EventID); err != nil {
			return nil, ErrInvalidID
		}
		if _, err := uuid.Parse(sr.SeatID); err != nil {
			return nil, ErrInvalidID
		}
	}

	orderID, err := s.orders.Checkout(ctx, userID, seats, OrderTTL, idempotencyKey)
	switch {
	case err == nil:
	case errors.Is(err, repository.ErrNotFound):
		return nil, ErrNotFound
	case errors.Is(err, repository.ErrSeatNotHeld):
		return nil, ErrSeatNotHeld
	case errors.Is(err, repository.ErrMixedCurrency):
		return nil, ErrMixedCurrency
	default:
		return nil, err
	}

	return s.orders.GetDetail(ctx, orderID, userID)
}

func (s *OrderService) Get(ctx context.Context, userID, orderID string) (*domain.OrderDetail, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidID
	}
	if _, err := uuid.Parse(orderID); err != nil {
		return nil, ErrInvalidID
	}
	d, err := s.orders.GetDetail(ctx, orderID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	return d, err
}

func (s *OrderService) List(ctx context.Context, userID string) ([]domain.Order, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidID
	}
	return s.orders.ListByUser(ctx, userID)
}

// Pay bir siparisi oder. fail=true ise mock saglayici odemeyi reddeder.
func (s *OrderService) Pay(ctx context.Context, userID, orderID string, fail bool) (*domain.Payment, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidID
	}
	if _, err := uuid.Parse(orderID); err != nil {
		return nil, ErrInvalidID
	}

	p, err := s.orders.Pay(ctx, orderID, userID, fail)
	switch {
	case err == nil:
		// Odeme basariliysa koltuklar artik 'sold'. Redis hold anahtarlari
		// TTL ile kendiliginden silinecegi icin ekstra temizlik gerekmiyor.
		return &p, nil
	case errors.Is(err, repository.ErrNotFound):
		return nil, ErrNotFound
	case errors.Is(err, repository.ErrOrderNotPayable):
		return nil, ErrOrderNotPayable
	case errors.Is(err, repository.ErrSeatLost):
		return nil, ErrSeatLost
	default:
		return nil, err
	}
}
