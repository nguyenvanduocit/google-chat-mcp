# Google Chat MCP

A Model Context Protocol (MCP) server for Google Chat, enabling AI assistants to interact with Google Chat spaces, messages, and members.

## Features

- **Spaces**: List and get details of Google Chat spaces
- **Messages**: List, get, send, and delete messages
- **Members**: List and get members of spaces

## Prerequisites

1. A Google Cloud project with the Google Chat API enabled
2. A service account with the necessary permissions
3. Service account JSON key file

## Setup

### 1. Create a Service Account

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the **Google Chat API**
4. Go to **IAM & Admin** > **Service Accounts**
5. Create a new service account
6. Grant the service account the necessary roles for Google Chat
7. Create and download a JSON key file

### 2. Configure the Service Account for Google Chat

For the Chat API to work with a service account, you need to:

1. Set up domain-wide delegation for the service account
2. Or add the service account as a member to Chat spaces

### 3. Set Environment Variables

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

Or create a `.env` file:

```
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

## Usage

### Build

```bash
go build -o google-chat-mcp .
```

### Run (STDIO mode)

```bash
./google-chat-mcp
```

### Run (HTTP mode)

```bash
./google-chat-mcp --http_port 3003
```

### Run with env file

```bash
./google-chat-mcp --env .env
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

### Members

- `google_chat_list_members` - List members of a space
- `google_chat_get_member` - Get details of a specific member

## License

MIT
