package migrations

import (
	"context"

	"github.com/distributed-programming-2026/go-sdk/pkg/migrator"
	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"
)

func Version1792005017(client mysql.ClientContext) migrator.Migration {
	return &version1792005017{client: client}
}

type version1792005017 struct {
	client mysql.ClientContext
}

func (v version1792005017) Version() int64 {
	return 1792005017
}

func (v version1792005017) Description() string {
	return "Create users table"
}

func (v version1792005017) Up(ctx context.Context) error {
	const query = `
		CREATE TABLE IF NOT EXISTS users (
			id CHAR(36) CHARACTER SET ascii NOT NULL PRIMARY KEY,
			first_name VARCHAR(100) NOT NULL,
			last_name VARCHAR(100) NOT NULL,
			email VARCHAR(254) NOT NULL,
			status VARCHAR(16) CHARACTER SET ascii NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL
		) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`
	_, err := v.client.ExecContext(ctx, "create-users-table", query)
	return err
}
