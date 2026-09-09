package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/emrerkus/ticket-sale-backend/internal/domain"
)

// ErrEmailTaken: bu email ile zaten bir kullanici var.
var ErrEmailTaken = errors.New("repository: email zaten kayitli")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create yeni bir kullanici ekler. Email cakisirsa ErrEmailTaken doner.
// (users_email_lower_key UNIQUE index'i sayesinde yaris durumunda da guvenli:
//
//	iki es zamanli kayit denemesinden biri DB seviyesinde reddedilir.)
func (r *UserRepository) Create(ctx context.Context, email, passwordHash string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if err != nil {
		// 23505 = unique_violation (users_email_lower_key)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, ErrEmailTaken
		}
		return domain.User{}, fmt.Errorf("kullanici olusturulamadi: %w", err)
	}
	return u, nil
}

// GetByEmail login icin: email'e gore kullaniciyi (hash dahil) getirir.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, created_at
		FROM users WHERE lower(email) = lower($1)`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("kullanici okunamadi: %w", err)
	}
	return u, nil
}

// GetByID auth middleware sonrasi "ben kimim" icin.
func (r *UserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, created_at
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("kullanici okunamadi: %w", err)
	}
	return u, nil
}
