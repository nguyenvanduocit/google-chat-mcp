package services

import (
	"context"
	"fmt"

	"github.com/nguyenvanduocit/google-chat-mcp/auth"
	"google.golang.org/api/chat/v1"
	"google.golang.org/api/option"
)

// ChatServiceFromContext creates a per-user Chat service from the authenticated HTTP client in context.
func ChatServiceFromContext(ctx context.Context) (*chat.Service, error) {
	client := auth.HTTPClientFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("no authenticated HTTP client in context — user must authenticate first")
	}

	return chat.NewService(ctx, option.WithHTTPClient(client))
}
