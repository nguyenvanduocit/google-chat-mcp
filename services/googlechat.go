package services

import (
	"context"
	"fmt"
	"os"
	"sync"

	"google.golang.org/api/chat/v1"
	"google.golang.org/api/option"
)

var ChatService = sync.OnceValue(func() *chat.Service {
	ctx := context.Background()

	credentialsFile := os.Getenv("GOOGLE_CREDENTIALS_FILE")
	if credentialsFile == "" {
		panic("GOOGLE_CREDENTIALS_FILE environment variable must be set")
	}

	tokenFile := os.Getenv("GOOGLE_TOKEN_FILE")
	if tokenFile == "" {
		panic("GOOGLE_TOKEN_FILE environment variable must be set")
	}

	client := GoogleHttpClient(tokenFile, credentialsFile)

	service, err := chat.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		panic(fmt.Sprintf("Failed to create Google Chat service: %v", err))
	}

	return service
})
