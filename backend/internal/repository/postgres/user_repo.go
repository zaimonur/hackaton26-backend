package postgres

import (
	"context"
	"drewisy/internal/domain"

	"github.com/jmoiron/sqlx"
)

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) domain.UserRepository {
	return &userRepository{db}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	query := `INSERT INTO users (email, password_hash, role, created_at) 
              VALUES ($1, $2, $3, NOW()) RETURNING id, created_at`
	return r.db.QueryRowContext(ctx, query, u.Email, u.PasswordHash, u.Role).
		Scan(&u.ID, &u.CreatedAt)
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	query := `SELECT id, email, password_hash, role, created_at FROM users WHERE email = $1`
	err := r.db.GetContext(ctx, &u, query, email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
