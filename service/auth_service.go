package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"order-management/apperror"
	"order-management/auth"
	"order-management/model"
	"order-management/repository"
)

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.TokenResponse, error)
	SeedAdmin(ctx context.Context, email, password string) error
}

type authService struct {
	users     repository.UserRepository
	tokens    *auth.TokenManager
	dummyHash []byte
}

func NewAuthService(users repository.UserRepository, tokens *auth.TokenManager) AuthService {
	dummy, err := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return &authService{users: users, tokens: tokens, dummyHash: dummy}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *authService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	return s.createUser(ctx, req.Email, req.Password, model.RoleCustomer)
}

func (s *authService) createUser(ctx context.Context, email, password string, role model.Role) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.Internal().WithCause(err)
	}
	user := &model.User{Email: normalizeEmail(email), PasswordHash: string(hash), Role: role}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (*model.TokenResponse, error) {
	user, err := s.users.GetByEmail(ctx, normalizeEmail(req.Email))
	if err != nil {
		if ae, ok := apperror.As(err); ok && ae.Status == http.StatusNotFound {
			_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(req.Password))
			return nil, apperror.Unauthorized("invalid credentials")
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, apperror.Unauthorized("invalid credentials")
	}
	token, expires, err := s.tokens.Issue(user)
	if err != nil {
		return nil, apperror.Internal().WithCause(err)
	}
	return &model.TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expires).Seconds()),
	}, nil
}

func (s *authService) SeedAdmin(ctx context.Context, email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	if len(password) < 12 {
		return apperror.BadRequest("ADMIN_PASSWORD must be at least 12 characters")
	}
	_, err := s.users.GetByEmail(ctx, normalizeEmail(email))
	if err == nil {
		return nil
	}
	if ae, ok := apperror.As(err); !ok || ae.Status != http.StatusNotFound {
		return err
	}
	_, err = s.createUser(ctx, email, password, model.RoleAdmin)
	return err
}
