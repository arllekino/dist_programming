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
	return "Create credentials table"
}

func (v version1792005017) Up(ctx context.Context) error {
	const query = `
		CREATE TABLE IF NOT EXISTS credentials (
			user_id BINARY(16) NOT NULL PRIMARY KEY,
			login VARCHAR(64) CHARACTER SET ascii NOT NULL,
			password_hash VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
			created_at DATETIME(6) NOT NULL,
			password_changed_at DATETIME(6) NOT NULL,
			UNIQUE KEY credentials_login_unique (login)
		) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`
	_, err := v.client.ExecContext(ctx, "create-credentials-table", query)
	return err
}
