package auth

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

type contextKey string

const (
	googleTokenKey contextKey = "google_oauth_token"
	httpClientKey  contextKey = "google_http_client"
)

func ContextWithGoogleToken(ctx context.Context, token *oauth2.Token) context.Context {
	return context.WithValue(ctx, googleTokenKey, token)
}

func GoogleTokenFromContext(ctx context.Context) *oauth2.Token {
	token, _ := ctx.Value(googleTokenKey).(*oauth2.Token)
	return token
}

func ContextWithHTTPClient(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, httpClientKey, client)
}

func HTTPClientFromContext(ctx context.Context) *http.Client {
	client, _ := ctx.Value(httpClientKey).(*http.Client)
	return client
}
