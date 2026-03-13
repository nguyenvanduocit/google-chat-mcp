# Google Chat MCP - build targets

# Build the MCP server
build:
    go build -o bin/google-chat-mcp .

# Build the CLI binary
build-cli:
    go build -o bin/google-chat-cli ./cmd/cli/

# Install the MCP server to $GOPATH/bin
install:
    go install .

# Install the CLI binary to $GOPATH/bin
install-cli:
    go install ./cmd/cli/

# Run tests
test:
    go test ./...

# Tidy dependencies
tidy:
    go mod tidy
