package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/google-chat-mcp/services"
	"google.golang.org/api/chat/v1"
)

type ListMessagesInput struct {
	SpaceName string `json:"space_name" validate:"required"`
	PageSize  int    `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

type GetMessageInput struct {
	MessageName string `json:"message_name" validate:"required"`
}

type SendMessageInput struct {
	SpaceName string `json:"space_name" validate:"required"`
	Text      string `json:"text" validate:"required"`
	ThreadKey string `json:"thread_key,omitempty"`
}

type DeleteMessageInput struct {
	MessageName string `json:"message_name" validate:"required"`
}

type UploadAttachmentInput struct {
	SpaceName string `json:"space_name" validate:"required"`
	FilePath  string `json:"file_path" validate:"required"`
	Text      string `json:"text,omitempty"`
	ThreadKey string `json:"thread_key,omitempty"`
}

func RegisterMessagesTool(s *server.MCPServer) {
	listMessagesTool := mcp.NewTool("google_chat_list_messages",
		mcp.WithDescription("List messages in a Google Chat space"),
		mcp.WithString("space_name", mcp.Required(), mcp.Description("The resource name of the space (e.g., 'spaces/AAAA1234')")),
		mcp.WithNumber("page_size", mcp.Description("Maximum number of messages to return (default 25, max 1000)")),
		mcp.WithString("page_token", mcp.Description("Page token for pagination")),
	)
	s.AddTool(listMessagesTool, mcp.NewTypedToolHandler(listMessagesHandler))

	getMessageTool := mcp.NewTool("google_chat_get_message",
		mcp.WithDescription("Get a specific message from Google Chat"),
		mcp.WithString("message_name", mcp.Required(), mcp.Description("The resource name of the message (e.g., 'spaces/AAAA1234/messages/BBBB5678')")),
	)
	s.AddTool(getMessageTool, mcp.NewTypedToolHandler(getMessageHandler))

	sendMessageTool := mcp.NewTool("google_chat_send_message",
		mcp.WithDescription("Send a text message to a Google Chat space"),
		mcp.WithString("space_name", mcp.Required(), mcp.Description("The resource name of the space (e.g., 'spaces/AAAA1234')")),
		mcp.WithString("text", mcp.Required(), mcp.Description("The text content of the message")),
		mcp.WithString("thread_key", mcp.Description("Thread key for replying to a specific thread")),
	)
	s.AddTool(sendMessageTool, mcp.NewTypedToolHandler(sendMessageHandler))

	deleteMessageTool := mcp.NewTool("google_chat_delete_message",
		mcp.WithDescription("Delete a message from Google Chat"),
		mcp.WithString("message_name", mcp.Required(), mcp.Description("The resource name of the message to delete (e.g., 'spaces/AAAA1234/messages/BBBB5678')")),
	)
	s.AddTool(deleteMessageTool, mcp.NewTypedToolHandler(deleteMessageHandler))

	uploadAttachmentTool := mcp.NewTool("google_chat_upload_attachment",
		mcp.WithDescription("Upload a file attachment to a Google Chat space"),
		mcp.WithString("space_name", mcp.Required(), mcp.Description("The resource name of the space (e.g., 'spaces/AAAA1234')")),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("The local file path to upload")),
		mcp.WithString("text", mcp.Description("Optional text message to accompany the attachment")),
		mcp.WithString("thread_key", mcp.Description("Thread key for replying to a specific thread")),
	)
	s.AddTool(uploadAttachmentTool, mcp.NewTypedToolHandler(uploadAttachmentHandler))
}

func listMessagesHandler(ctx context.Context, request mcp.CallToolRequest, input ListMessagesInput) (*mcp.CallToolResult, error) {
	service, err := services.ChatServiceFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat service: %v", err)
	}

	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 25
	}

	call := service.Spaces.Messages.List(input.SpaceName).
		PageSize(int64(pageSize)).
		OrderBy("createTime desc")
	if input.PageToken != "" {
		call = call.PageToken(input.PageToken)
	}

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %v", err)
	}

	if len(resp.Messages) == 0 {
		return mcp.NewToolResultText("No messages found in this space."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d messages:\n\n", len(resp.Messages)))

	for _, msg := range resp.Messages {
		sb.WriteString(formatMessage(msg))
		sb.WriteString("\n---\n")
	}

	if resp.NextPageToken != "" {
		sb.WriteString(fmt.Sprintf("\nNext Page Token: %s\n", resp.NextPageToken))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func getMessageHandler(ctx context.Context, request mcp.CallToolRequest, input GetMessageInput) (*mcp.CallToolResult, error) {
	service, err := services.ChatServiceFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat service: %v", err)
	}

	msg, err := service.Spaces.Messages.Get(input.MessageName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %v", err)
	}

	return mcp.NewToolResultText(formatMessage(msg)), nil
}

func sendMessageHandler(ctx context.Context, request mcp.CallToolRequest, input SendMessageInput) (*mcp.CallToolResult, error) {
	service, err := services.ChatServiceFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat service: %v", err)
	}

	message := &chat.Message{
		Text: input.Text,
	}

	if input.ThreadKey != "" {
		message.Thread = &chat.Thread{
			ThreadKey: input.ThreadKey,
		}
	}

	msg, err := service.Spaces.Messages.Create(input.SpaceName, message).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("Message sent successfully!\n\n")
	sb.WriteString(formatMessage(msg))

	return mcp.NewToolResultText(sb.String()), nil
}

func deleteMessageHandler(ctx context.Context, request mcp.CallToolRequest, input DeleteMessageInput) (*mcp.CallToolResult, error) {
	service, err := services.ChatServiceFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat service: %v", err)
	}

	_, err = service.Spaces.Messages.Delete(input.MessageName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to delete message: %v", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Message %s deleted successfully!", input.MessageName)), nil
}

func uploadAttachmentHandler(ctx context.Context, request mcp.CallToolRequest, input UploadAttachmentInput) (*mcp.CallToolResult, error) {
	service, err := services.ChatServiceFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat service: %v", err)
	}

	// Open the file
	file, err := os.Open(input.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Get file info for the filename
	filename := filepath.Base(input.FilePath)

	// Create the message with attachment
	message := &chat.Message{}
	if input.Text != "" {
		message.Text = input.Text
	}

	if input.ThreadKey != "" {
		message.Thread = &chat.Thread{
			ThreadKey: input.ThreadKey,
		}
	}

	// Upload attachment using Media.Upload
	// Parent must be just the space name: spaces/{space}
	uploadCall := service.Media.Upload(input.SpaceName, &chat.UploadAttachmentRequest{
		Filename: filename,
	})
	uploadCall.Media(file)

	uploadResp, err := uploadCall.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to upload attachment: %v", err)
	}

	// Add attachment reference to the message
	message.Attachment = []*chat.Attachment{
		{
			AttachmentDataRef: uploadResp.AttachmentDataRef,
		},
	}

	// Send the message
	msg, err := service.Spaces.Messages.Create(input.SpaceName, message).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to send message with attachment: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("Attachment uploaded and message sent successfully!\n\n")
	sb.WriteString(fmt.Sprintf("File: %s\n", filename))
	sb.WriteString(formatMessage(msg))

	return mcp.NewToolResultText(sb.String()), nil
}

func formatMessage(msg *chat.Message) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name: %s\n", msg.Name))
	if msg.Sender != nil {
		sb.WriteString(fmt.Sprintf("Sender: %s (%s)\n", msg.Sender.DisplayName, msg.Sender.Name))
	}
	sb.WriteString(fmt.Sprintf("Text: %s\n", msg.Text))
	sb.WriteString(fmt.Sprintf("Created: %s\n", msg.CreateTime))
	if msg.LastUpdateTime != "" && msg.LastUpdateTime != msg.CreateTime {
		sb.WriteString(fmt.Sprintf("Edited: %s\n", msg.LastUpdateTime))
	}
	if msg.Thread != nil && msg.Thread.Name != "" {
		sb.WriteString(fmt.Sprintf("Thread: %s\n", msg.Thread.Name))
	}
	if len(msg.Attachment) > 0 {
		sb.WriteString(fmt.Sprintf("Attachments: %d\n", len(msg.Attachment)))
		for _, att := range msg.Attachment {
			sb.WriteString(fmt.Sprintf("  - %s (%s)\n", att.Name, att.ContentType))
		}
	}
	return sb.String()
}
