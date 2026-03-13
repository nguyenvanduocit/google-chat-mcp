# Google Chat MCP

A Model Context Protocol (MCP) server for Google Chat, enabling AI assistants to interact with Google Chat spaces, messages, and members.

## Features

- **Spaces**: List and get details of Google Chat spaces
- **Messages**: List, get, send, and delete messages
- **Members**: List and get members of spaces
- **Attachments**: Upload file attachments to messages

## Prerequisites

1. A Google Cloud project with the Google Chat API enabled
2. OAuth 2.0 Client ID credentials (Desktop app type)

## Setup

### 1. Create OAuth 2.0 Credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the **Google Chat API**
4. Go to **APIs & Services** > **Credentials**
5. Click **Create Credentials** > **OAuth client ID**
6. Select **Desktop app** as the application type
7. Download the JSON file and save it as `google-credentials.json`

### 2. Generate OAuth Token

Use the included token generation tool:

```bash
# Build the token generator
go build -o get-google-token ./scripts/get-google-token

# Run it with your credentials file
./get-google-token \
  -credentials /path/to/google-credentials.json \
  -token /path/to/google-token.json
```

This will:
1. Open your browser for Google authentication
2. Ask you to authorize the required Chat API scopes
3. Save the OAuth token to the specified path

### 3. Set Environment Variables

```bash
export GOOGLE_CREDENTIALS_FILE=/path/to/google-credentials.json
export GOOGLE_TOKEN_FILE=/path/to/google-token.json
```

Or create a `.env` file:

```
GOOGLE_CREDENTIALS_FILE=/path/to/google-credentials.json
GOOGLE_TOKEN_FILE=/path/to/google-token.json
```

## Usage

### Install

```bash
go install github.com/nguyenvanduocit/google-chat-mcp@latest
```

### Build from source

```bash
go build -o google-chat-mcp .
```

### Run (STDIO mode)

```bash
./google-chat-mcp --env .env
```

### Run (HTTP mode)

```bash
./google-chat-mcp --env .env --http_port 3003
```

## MCP Configuration

Add to your MCP settings:

### STDIO mode
```json
{
  "mcpServers": {
    "google-chat": {
      "command": "/path/to/google-chat-mcp",
      "args": ["--env", "/path/to/.env"]
    }
  }
}
```

### HTTP mode
```json
{
  "mcpServers": {
    "google-chat": {
      "url": "http://localhost:3003/mcp"
    }
  }
}
```

## Available Tools

### Spaces

- `google_chat_list_spaces` - List all accessible Google Chat spaces
- `google_chat_get_space` - Get details of a specific space

### Messages

- `google_chat_list_messages` - List messages in a space
- `google_chat_get_message` - Get a specific message
- `google_chat_send_message` - Send a message to a space
- `google_chat_delete_message` - Delete a message
- `google_chat_upload_attachment` - Upload a file attachment to a space

### Members

- `google_chat_list_members` - List members of a space
- `google_chat_get_member` - Get details of a specific member

## CLI Usage

In addition to the MCP server, `google-chat-mcp` ships a standalone CLI binary (`google-chat-cli`) for direct terminal use — no MCP client needed.

### Installation

```bash
just install-cli
# or
go install github.com/nguyenvanduocit/google-chat-mcp/cmd/google-chat-cli@latest
```

### Quick Start

```bash
export GOOGLE_CREDENTIALS_FILE=/path/to/credentials.json
export GOOGLE_TOKEN_FILE=/path/to/token.json
# or
google-chat-cli --env .env <command> [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `list-spaces` | List Google Chat spaces |
| `get-space` | Get space details |
| `list-messages` | List messages in a space |
| `get-message` | Get a specific message |
| `send-message` | Send a message to a space |
| `delete-message` | Delete a message |
| `list-members` | List space members |
| `get-member` | Get member details |

### Examples

```bash
# List spaces
google-chat-cli list-spaces

# Send a message
google-chat-cli send-message --space spaces/XXXXXX --text "Hello from CLI!"

# List messages
google-chat-cli list-messages --space spaces/XXXXXX

# JSON output
google-chat-cli list-spaces --output json | jq '.[].name'
```

### Flags

Every command accepts:
- `--env string` — Path to `.env` file
- `--output string` — Output format: `text` (default) or `json`

## License

MIT
