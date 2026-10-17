package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"
	mysqldriver "github.com/go-sql-driver/mysql"

	"identity/internal/identity/domain"
)

const errDuplicateEntry = 1062

func NewUserRepository(ctx context.Context, client mysql.ClientContext) domain.UserRepository {
	return &userRepository{
		ctx:    ctx,
		client: client,
	}
}

type userRepository struct {
	ctx    context.Context
	client mysql.ClientContext
}

type userRow struct {
	ID        string    `db:"id"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
	Email     string    `db:"email"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (repo *userRepository) Find(id uuid.UUID) (domain.User, error) {
	const query = `
		SELECT
			id,
			first_name,
			last_name,
			email,
			status,
			created_at,
			updated_at
		FROM
			users
		WHERE
			id = ?
	`
	return repo.find("user.find", query, id)
}

func (repo *userRepository) FindForUpdate(id uuid.UUID) (domain.User, error) {
	const query = `
		SELECT
			id,
			first_name,
			last_name,
			email,
			status,
			created_at,
			updated_at
		FROM
			users
		WHERE
			id = ?
		FOR UPDATE
	`
	return repo.find("user.find_for_update", query, id)
}

func (repo *userRepository) find(queryID, query string, id uuid.UUID) (domain.User, error) {
	var row userRow
	err := repo.client.GetContext(repo.ctx, queryID, &row, query, id.String())
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	userID, err := uuid.Parse(row.ID)
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{
		ID: userID,
		Profile: domain.Profile{
			FirstName: row.FirstName,
			LastName:  row.LastName,
			Email:     row.Email,
		},
		Status:    domain.Status(row.Status),
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

func (repo *userRepository) Create(user domain.User) error {
	const query = `
		INSERT INTO users (
			id,
			first_name,
			last_name,
			email,
			status,
			created_at,
			updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?
		)
	`
	_, err := repo.client.ExecContext(repo.ctx, "user.create", query,
		user.ID.String(),
		user.Profile.FirstName,
		user.Profile.LastName,
		user.Profile.Email,
		string(user.Status),
		user.CreatedAt,
		user.UpdatedAt,
	)
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == errDuplicateEntry {
		return domain.ErrUserAlreadyExists
	}
	return err
}

func (repo *userRepository) Update(user domain.User) error {
	const query = `
		UPDATE users
		SET
			first_name = ?,
			last_name = ?,
			email = ?,
			status = ?,
			updated_at = ?
		WHERE
			id = ?
	`
	result, err := repo.client.ExecContext(repo.ctx, "user.update", query,
		user.Profile.FirstName,
		user.Profile.LastName,
		user.Profile.Email,
		string(user.Status),
		user.UpdatedAt,
		user.ID.String(),
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}
