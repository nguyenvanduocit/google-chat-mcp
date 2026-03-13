package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/google-chat-mcp/auth"
	"github.com/nguyenvanduocit/google-chat-mcp/services"
	"github.com/nguyenvanduocit/google-chat-mcp/tools"
)

func main() {
	envFile := flag.String("env", "", "Path to environment file (optional when environment variables are set directly)")
	httpPort := flag.String("http_port", "3003", "Port for HTTP server")
	sessionDir := flag.String("session_dir", "data/sessions", "Directory for storing user sessions")
	flag.Parse()

	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			fmt.Printf("Warning: Error loading env file %s: %v\n", *envFile, err)
		} else {
			fmt.Printf("Loaded environment variables from %s\n", *envFile)
		}
	}

	credentialsFile := os.Getenv("GOOGLE_CREDENTIALS_FILE")
	if credentialsFile == "" {
		fmt.Println("Configuration Error: GOOGLE_CREDENTIALS_FILE is required")
		fmt.Println()
		fmt.Println("Setup Instructions:")
		fmt.Println("1. Create OAuth credentials in Google Cloud Console")
		fmt.Println("2. Enable the Google Chat API for your project")
		fmt.Println("3. Set the environment variable:")
		fmt.Println()
		fmt.Println("   GOOGLE_CREDENTIALS_FILE=/path/to/google-credentials.json")
		fmt.Println()
		os.Exit(1)
	}

	mcpServer := server.NewMCPServer(
		"Google Chat MCP",
		"1.0.0",
		server.WithLogging(),
		server.WithRecovery(),
	)

	tools.RegisterSpacesTool(mcpServer)
	tools.RegisterMessagesTool(mcpServer)
	tools.RegisterMembersTool(mcpServer)

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", *httpPort)
	}

	authServer, err := auth.NewServer(mcpServer, auth.Config{
		CredentialsFile: credentialsFile,
		Scopes:          services.ListChatScopes(),
		SessionDir:      *sessionDir,
		BaseURL:         baseURL,
	})
	if err != nil {
		log.Fatalf("Failed to create auth server: %v", err)
	}

	fmt.Println()
	fmt.Println("Starting Google Chat MCP Server...")
	fmt.Printf("MCP endpoint: %s/mcp\n", baseURL)
	fmt.Printf("OAuth authorize: %s/authorize\n", baseURL)
	fmt.Printf("Session storage: %s\n", *sessionDir)
	fmt.Println()

	if err := authServer.Start(fmt.Sprintf(":%s", *httpPort)); err != nil && !isContextCanceled(err) {
		log.Fatalf("Server error: %v", err)
	}
}

func isContextCanceled(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return true
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "context canceled") ||
		strings.Contains(errMsg, "operation was canceled") ||
		strings.Contains(errMsg, "context deadline exceeded")
}
