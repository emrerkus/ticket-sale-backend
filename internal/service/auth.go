package service

import (
	"context"
	"errors"
	"strings"

	"github.com/emrerkus/ticket-sale-backend/internal/auth"
	"github.com/emrerkus/ticket-sale-backend/internal/domain"
	"github.com/emrerkus/ticket-sale-backend/internal/repository"
)

var (
	ErrEmailTaken         = errors.New("service: email zaten kayitli")
	ErrInvalidCredentials = errors.New("service: email veya sifre hatali")
	ErrWeakInput          = errors.New("service: email/sifre gecersiz")
)

type AuthService struct {
	users *repository.UserRepository
	tm    *auth.TokenManager
}

func NewAuthService(users *repository.UserRepository, tm *auth.TokenManager) *AuthService {
	return &AuthService{users: users, tm: tm}
}

// Register yeni kullanici olusturur ve giris yapmis gibi token doner.
func (s *AuthService) Register(ctx context.Context, email, password string) (*domain.AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !looksLikeEmail(email) || len(password) < 8 {
		return nil, ErrWeakInput
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	u, err := s.users.Create(ctx, email, hash)
	if errors.Is(err, repository.ErrEmailTaken) {
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, err
	}
	return s.issue(u)
}

// Login email+sifre dogrularsa token doner.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	u, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		// "kullanici yok" ile "sifre yanlis"i AYIRMIYORUZ -> saldirgan hangi
		// email'lerin kayitli oldugunu ogrenemez (user enumeration).
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return s.issue(u)
}

// Me: auth middleware'inden gecmis bir kullanicinin bilgisini doner.
func (s *AuthService) Me(ctx context.Context, userID string) (*domain.User, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	u.PasswordHash = ""
	return &u, nil
}

func (s *AuthService) issue(u domain.User) (*domain.AuthResult, error) {
	token, err := s.tm.Generate(u.ID)
	if err != nil {
		return nil, err
	}
	u.PasswordHash = "" // her ihtimale karsi
	return &domain.AuthResult{Token: token, User: u}, nil
}

func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-1 && strings.IndexByte(s[at+1:], '.') >= 0
}
