package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
)

// UserRepository implements user.Repository using a pgxpool connection pool.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository constructs a UserRepository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create inserts a new user row. The database assigns the UUID and timestamps.
func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	const q = `
		INSERT INTO users (email, phone_number, password_hash, first_name, last_name, role, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, version, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		u.Email,
		u.PhoneNumber,
		u.PasswordHash,
		u.FirstName,
		u.LastName,
		string(u.Role),
		string(u.Status),
	)

	if err := row.Scan(&u.ID, &u.Version, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return fmt.Errorf("scanning created user: %w", err)
	}
	return nil
}

// FindByID retrieves a user by primary key.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	const q = `
		SELECT id, email, phone_number, password_hash, first_name, last_name,
		       role, status, version, created_at, updated_at
		FROM users
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, fmt.Errorf("finding user by id: %w", err)
	}
	return u, nil
}

// FindByEmail retrieves a user by their unique email address.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	const q = `
		SELECT id, email, phone_number, password_hash, first_name, last_name,
		       role, status, version, created_at, updated_at
		FROM users
		WHERE email = $1`

	row := r.pool.QueryRow(ctx, q, email)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, fmt.Errorf("finding user by email: %w", err)
	}
	return u, nil
}

// Update persists changes to a user. The WHERE clause includes the current
// version to detect concurrent writes; the version is atomically incremented
// by the database. Returns user.ErrConflict when 0 rows are affected.
func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	const q = `
		UPDATE users
		SET email        = $1,
		    phone_number = $2,
		    first_name   = $3,
		    last_name    = $4,
		    role         = $5,
		    status       = $6,
		    version      = version + 1,
		    updated_at   = NOW()
		WHERE id = $7 AND version = $8
		RETURNING version, updated_at`

	row := r.pool.QueryRow(ctx, q,
		u.Email,
		u.PhoneNumber,
		u.FirstName,
		u.LastName,
		string(u.Role),
		string(u.Status),
		u.ID,
		u.Version,
	)

	if err := row.Scan(&u.Version, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.ErrConflict
		}
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

// Count returns the total number of user rows. Used during admin bootstrap to
// determine whether the system is uninitialized.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return n, nil
}

// scanUser reads a single user row from a pgx.Row.
func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	var roleStr, statusStr string

	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PhoneNumber,
		&u.PasswordHash,
		&u.FirstName,
		&u.LastName,
		&roleStr,
		&statusStr,
		&u.Version,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	u.Role = user.Role(roleStr)
	u.Status = user.Status(statusStr)
	return &u, nil
}
