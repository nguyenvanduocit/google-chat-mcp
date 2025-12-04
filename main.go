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
	"github.com/nguyenvanduocit/google-chat-mcp/tools"
)

func main() {
	envFile := flag.String("env", "", "Path to environment file (optional when environment variables are set directly)")
	httpPort := flag.String("http_port", "", "Port for HTTP server. If not provided, will use stdio")
	flag.Parse()

	// Load environment file if specified
	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			fmt.Printf("Warning: Error loading env file %s: %v\n", *envFile, err)
		} else {
			fmt.Printf("Loaded environment variables from %s\n", *envFile)
		}
	}

	// Check required environment variables
	requiredEnvs := []string{"GOOGLE_CREDENTIALS_FILE", "GOOGLE_TOKEN_FILE"}
	missingEnvs := []string{}
	for _, env := range requiredEnvs {
		if os.Getenv(env) == "" {
			missingEnvs = append(missingEnvs, env)
		}
	}

	if len(missingEnvs) > 0 {
		fmt.Println("Configuration Error: Missing required environment variables")
		fmt.Println()
		fmt.Println("Missing variables:")
		for _, env := range missingEnvs {
			fmt.Printf("  - %s\n", env)
		}
		fmt.Println()
		fmt.Println("Setup Instructions:")
		fmt.Println("1. Create OAuth credentials in Google Cloud Console")
		fmt.Println("2. Enable the Google Chat API for your project")
		fmt.Println("3. Run the get-google-token script to generate token")
		fmt.Println("4. Set the environment variables:")
		fmt.Println()
		fmt.Println("   GOOGLE_CREDENTIALS_FILE=/path/to/google-credentials.json")
		fmt.Println("   GOOGLE_TOKEN_FILE=/path/to/google-token.json")
		fmt.Println()
		os.Exit(1)
	}

	mcpServer := server.NewMCPServer(
		"Google Chat MCP",
		"1.0.0",
		server.WithLogging(),
		server.WithRecovery(),
	)

	// Register all Google Chat tools
	tools.RegisterSpacesTool(mcpServer)
	tools.RegisterMessagesTool(mcpServer)
	tools.RegisterMembersTool(mcpServer)

	if *httpPort != "" {
		fmt.Println()
		fmt.Println("Starting Google Chat MCP Server in HTTP mode...")
		fmt.Printf("Server will be available at: http://localhost:%s/mcp\n", *httpPort)
		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Println("Add the following to your MCP settings:")
		fmt.Println()
		fmt.Println("```json")
		fmt.Println("{")
		fmt.Println("  \"mcpServers\": {")
		fmt.Println("    \"google-chat\": {")
		fmt.Printf("      \"url\": \"http://localhost:%s/mcp\"\n", *httpPort)
		fmt.Println("    }")
		fmt.Println("  }")
		fmt.Println("}")
		fmt.Println("```")
		fmt.Println()

		httpServer := server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath("/mcp"))
		if err := httpServer.Start(fmt.Sprintf(":%s", *httpPort)); err != nil && !isContextCanceled(err) {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		if err := server.ServeStdio(mcpServer); err != nil && !isContextCanceled(err) {
			log.Fatalf("Server error: %v", err)
		}
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
