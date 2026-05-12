package usecase

import (
	"context"
	"errors"
	"os"
	"time"

	"drewisy/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type userUsecase struct {
	repo domain.UserRepository
}

func NewUserUsecase(r domain.UserRepository) domain.UserUsecase {
	return &userUsecase{repo: r}
}

func (u *userUsecase) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.UserResponse, error) {
	if req.Role != "admin" && req.Role != "customer" && req.Role != "seller" {
		return nil, errors.New("geçersiz rol: admin, customer veya seller olmalı")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Role:         req.Role,
	}

	if err := u.repo.Create(ctx, user); err != nil {
		return nil, errors.New("bu e-posta zaten kayıtlı")
	}

	return &domain.UserResponse{
		ID:    user.ID,
		Email: user.Email,
		Role:  user.Role,
	}, nil
}

func (u *userUsecase) Login(ctx context.Context, req *domain.LoginRequest) (*domain.LoginResponse, error) {
	user, err := u.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("geçersiz e-posta veya şifre")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, errors.New("geçersiz e-posta veya şifre")
	}

	claims := jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "drewisy_hackathon_super_secret"
	}

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, errors.New("token üretilemedi")
	}

	return &domain.LoginResponse{
		Token: tokenString,
		User: domain.UserResponse{
			ID:    user.ID,
			Email: user.Email,
			Role:  user.Role,
		},
	}, nil
}
