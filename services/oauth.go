package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/chat/v1"
)

// tokenFromFile retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// ListChatScopes returns all Google Chat API scopes
func ListChatScopes() []string {
	return []string{
		chat.ChatSpacesScope,
		chat.ChatSpacesReadonlyScope,
		chat.ChatMessagesScope,
		chat.ChatMessagesCreateScope,
		chat.ChatMessagesReadonlyScope,
		chat.ChatMembershipsScope,
		chat.ChatMembershipsReadonlyScope,
	}
}

// GoogleHttpClient creates an HTTP client with OAuth credentials
func GoogleHttpClient(tokenFile string, credentialsFile string) *http.Client {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		panic(fmt.Sprintf("failed to read token file: %v", err))
	}

	ctx := context.Background()
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, ListChatScopes()...)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	return config.Client(ctx, tok)
}
