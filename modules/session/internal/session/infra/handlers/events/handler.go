package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/event"

	authevents "session/api/subscriptions/authevents"
	"session/internal/session/app"
)

const passwordChangedType = "PasswordChanged"

func RoutingKeys() []string {
	return []string{"auth." + passwordChangedType}
}

type handler struct {
	svc    *app.Service
	logger *slog.Logger
}

func NewHandler(svc *app.Service, logger *slog.Logger) event.Handler {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

func (h *handler) Handle(ctx context.Context, envelope event.Envelope) error {
	if envelope.Producer() != "auth" || envelope.Type() != passwordChangedType {
		return fmt.Errorf("unsupported event %s.%s", envelope.Producer(), envelope.Type())
	}
	var changed authevents.PasswordChanged
	err := envelope.UnmarshalPayload(&changed)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(changed.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID in PasswordChanged: %w", err)
	}
	if changed.ChangedAt == 0 {
		return fmt.Errorf("PasswordChanged for %s has no ChangedAt", userID)
	}
	revoked, err := h.svc.RevokeUserSessions(ctx, userID, changed.KeepSessionID, time.UnixMilli(changed.ChangedAt))
	if err != nil {
		return err
	}
	h.logger.InfoContext(ctx, "sessions revoked after password change",
		slog.String("user_id", userID.String()),
		slog.Int("revoked", revoked),
	)
	return nil
}
