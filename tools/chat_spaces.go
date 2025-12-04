package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/google-chat-mcp/services"
)

type ListSpacesInput struct {
	PageSize  int    `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

type GetSpaceInput struct {
	SpaceName string `json:"space_name" validate:"required"`
}

func RegisterSpacesTool(s *server.MCPServer) {
	listSpacesTool := mcp.NewTool("google_chat_list_spaces",
		mcp.WithDescription("List all Google Chat spaces (rooms and DMs) accessible by the service account"),
		mcp.WithNumber("page_size", mcp.Description("Maximum number of spaces to return (default 100, max 1000)")),
		mcp.WithString("page_token", mcp.Description("Page token for pagination")),
	)
	s.AddTool(listSpacesTool, mcp.NewTypedToolHandler(listSpacesHandler))

	getSpaceTool := mcp.NewTool("google_chat_get_space",
		mcp.WithDescription("Get details of a specific Google Chat space"),
		mcp.WithString("space_name", mcp.Required(), mcp.Description("The resource name of the space (e.g., 'spaces/AAAA1234')")),
	)
	s.AddTool(getSpaceTool, mcp.NewTypedToolHandler(getSpaceHandler))
}

func listSpacesHandler(ctx context.Context, request mcp.CallToolRequest, input ListSpacesInput) (*mcp.CallToolResult, error) {
	service := services.ChatService()

	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	call := service.Spaces.List().PageSize(int64(pageSize))
	if input.PageToken != "" {
		call = call.PageToken(input.PageToken)
	}

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list spaces: %v", err)
	}

	if len(resp.Spaces) == 0 {
		return mcp.NewToolResultText("No spaces found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d spaces:\n\n", len(resp.Spaces)))

	for _, space := range resp.Spaces {
		sb.WriteString(fmt.Sprintf("Name: %s\n", space.Name))
		sb.WriteString(fmt.Sprintf("Display Name: %s\n", space.DisplayName))
		sb.WriteString(fmt.Sprintf("Type: %s\n", space.Type))
		if space.SpaceType != "" {
			sb.WriteString(fmt.Sprintf("Space Type: %s\n", space.SpaceType))
		}
		sb.WriteString("\n")
	}

	if resp.NextPageToken != "" {
		sb.WriteString(fmt.Sprintf("Next Page Token: %s\n", resp.NextPageToken))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func getSpaceHandler(ctx context.Context, request mcp.CallToolRequest, input GetSpaceInput) (*mcp.CallToolResult, error) {
	service := services.ChatService()

	space, err := service.Spaces.Get(input.SpaceName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get space: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("Space Details:\n\n")
	sb.WriteString(fmt.Sprintf("Name: %s\n", space.Name))
	sb.WriteString(fmt.Sprintf("Display Name: %s\n", space.DisplayName))
	sb.WriteString(fmt.Sprintf("Type: %s\n", space.Type))
	if space.SpaceType != "" {
		sb.WriteString(fmt.Sprintf("Space Type: %s\n", space.SpaceType))
	}
	if space.SingleUserBotDm {
		sb.WriteString("Single User Bot DM: true\n")
	}
	if space.Threaded {
		sb.WriteString("Threaded: true\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}
