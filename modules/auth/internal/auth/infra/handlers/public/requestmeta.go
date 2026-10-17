package public

import (
	"context"
	"net/http"
	"strings"
)

type requestMetaKey struct{}

type requestMeta struct {
	token     string
	userAgent string
}

func withRequestMeta(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		meta := requestMeta{userAgent: r.UserAgent()}
		scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
		if found && strings.EqualFold(scheme, "Bearer") {
			meta.token = strings.TrimSpace(token)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestMetaKey{}, meta)))
	})
}

func metaFrom(ctx context.Context) requestMeta {
	meta, _ := ctx.Value(requestMetaKey{}).(requestMeta)
	return meta
}
