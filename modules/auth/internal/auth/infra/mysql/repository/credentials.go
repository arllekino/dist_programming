package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"
	mysqldriver "github.com/go-sql-driver/mysql"

	"auth/internal/auth/domain"
)

const errDuplicateEntry = 1062

func NewCredentialsRepository(ctx context.Context, client mysql.ClientContext) domain.CredentialsRepository {
	return &credentialsRepository{
		ctx:    ctx,
		client: client,
	}
}

type credentialsRepository struct {
	ctx    context.Context
	client mysql.ClientContext
}

type credentialsRow struct {
	UserID            []byte    `db:"user_id"`
	Login             string    `db:"login"`
	PasswordHash      string    `db:"password_hash"`
	CreatedAt         time.Time `db:"created_at"`
	PasswordChangedAt time.Time `db:"password_changed_at"`
}

func (repo *credentialsRepository) FindByLogin(login domain.Login) (domain.Credentials, error) {
	const query = `
		SELECT
			user_id,
			login,
			password_hash,
			created_at,
			password_changed_at
		FROM
			credentials
		WHERE
			login = ?
	`
	return repo.find("credentials.find_by_login", query, string(login))
}

func (repo *credentialsRepository) FindByUserID(userID uuid.UUID) (domain.Credentials, error) {
	const query = `
		SELECT
			user_id,
			login,
			password_hash,
			created_at,
			password_changed_at
		FROM
			credentials
		WHERE
			user_id = ?
	`
	return repo.find("credentials.find_by_user_id", query, userID[:])
}

func (repo *credentialsRepository) FindByUserIDForUpdate(userID uuid.UUID) (domain.Credentials, error) {
	const query = `
		SELECT
			user_id,
			login,
			password_hash,
			created_at,
			password_changed_at
		FROM
			credentials
		WHERE
			user_id = ?
		FOR UPDATE
	`
	return repo.find("credentials.find_by_user_id_for_update", query, userID[:])
}

func (repo *credentialsRepository) find(queryID, query string, arg any) (domain.Credentials, error) {
	var row credentialsRow
	err := repo.client.GetContext(repo.ctx, queryID, &row, query, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Credentials{}, domain.ErrCredentialsNotFound
	}
	if err != nil {
		return domain.Credentials{}, err
	}
	userID, err := parseUserID(row.UserID)
	if err != nil {
		return domain.Credentials{}, err
	}
	return domain.Credentials{
		UserID:            userID,
		Login:             domain.Login(row.Login),
		PasswordHash:      domain.PasswordHash(row.PasswordHash),
		CreatedAt:         row.CreatedAt.UTC(),
		PasswordChangedAt: row.PasswordChangedAt.UTC(),
	}, nil
}

func (repo *credentialsRepository) Create(c domain.Credentials) error {
	const query = `
		INSERT INTO credentials (
			user_id,
			login,
			password_hash,
			created_at,
			password_changed_at
		) VALUES (
			?, ?, ?, ?, ?
		)
	`
	_, err := repo.client.ExecContext(repo.ctx, "credentials.create", query,
		c.UserID[:],
		string(c.Login),
		string(c.PasswordHash),
		c.CreatedAt,
		c.PasswordChangedAt,
	)
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == errDuplicateEntry {
		return domain.ErrLoginTaken
	}
	return err
}

func (repo *credentialsRepository) Update(c domain.Credentials) error {
	const query = `
		UPDATE credentials
		SET
			password_hash = ?,
			password_changed_at = ?
		WHERE
			user_id = ?
	`
	result, err := repo.client.ExecContext(repo.ctx, "credentials.update", query,
		string(c.PasswordHash),
		c.PasswordChangedAt,
		c.UserID[:],
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrCredentialsNotFound
	}
	return nil
}

func parseUserID(raw []byte) (uuid.UUID, error) {
	if len(raw) != len(uuid.UUID{}) {
		return uuid.Nil(), fmt.Errorf("user ID must be %d bytes, got %d", len(uuid.UUID{}), len(raw))
	}
	return uuid.UUID(raw), nil
}
