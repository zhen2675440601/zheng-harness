package server

import (
	"context"
	"net/http"

	"zheng-harness/internal/service"
)

type contextKey string

const authSubjectContextKey contextKey = "auth_subject"

func (a *API) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			a.writeStructuredError(w, r, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		claims, err := validateJWT(token, a.JWTSecret, a.now())
		if err != nil {
			a.writeStructuredError(w, r, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), authSubjectContextKey, claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) authService() *service.AuthService {
	if a.AuthService != nil {
		return a.AuthService
	}
	a.AuthService = service.NewAuthService(service.AuthDependencies{UserStore: a.UserStore, JWTSecret: a.JWTSecret, Clock: a.Clock})
	return a.AuthService
}
