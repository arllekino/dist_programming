package clients

import (
	"fmt"

	"auth/internal/auth/app"
)

func unavailable(service string, err error) error {
	return fmt.Errorf("%w: %s: %w", app.ErrDependencyUnavailable, service, err)
}
