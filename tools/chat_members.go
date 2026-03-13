package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyenvanduocit/google-chat-mcp/services"
)

type ListMembersInput struct {
	SpaceName string `json:"space_name" validate:"required"`
	PageSize  int    `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

type GetMemberInput struct {
	MemberName string `json:"member_name" validate:"required"`
}

func RegisterMembersTool(s *server.MCPServer) {
	listMembersTool := mcp.NewTool("google_chat_list_members",
		mcp.WithDescription("List members of a Google Chat space"),
		mcp.WithString("space_name", mcp.Required(), mcp.Description("The resource name of the space (e.g., 'spaces/AAAA1234')")),
		mcp.WithNumber("page_size", mcp.Description("Maximum number of members to return (default 100, max 1000)")),
		mcp.WithString("page_token", mcp.Description("Page token for pagination")),
	)
	s.AddTool(listMembersTool, mcp.NewTypedToolHandler(listMembersHandler))

	getMemberTool := mcp.NewTool("google_chat_get_member",
		mcp.WithDescription("Get details of a specific member in a Google Chat space"),
		mcp.WithString("member_name", mcp.Required(), mcp.Description("The resource name of the member (e.g., 'spaces/AAAA1234/members/CCCC9012')")),
	)
	s.AddTool(getMemberTool, mcp.NewTypedToolHandler(getMemberHandler))
}

func listMembersHandler(ctx context.Context, request mcp.CallToolRequest, input ListMembersInput) (*mcp.CallToolResult, error) {
	service, err := services.ChatServiceFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat service: %v", err)
	}

	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	call := service.Spaces.Members.List(input.SpaceName).PageSize(int64(pageSize))
	if input.PageToken != "" {
		call = call.PageToken(input.PageToken)
	}

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %v", err)
	}

	if len(resp.Memberships) == 0 {
		return mcp.NewToolResultText("No members found in this space."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d members:\n\n", len(resp.Memberships)))

	for _, member := range resp.Memberships {
		sb.WriteString(fmt.Sprintf("Name: %s\n", member.Name))
		sb.WriteString(fmt.Sprintf("State: %s\n", member.State))
		sb.WriteString(fmt.Sprintf("Role: %s\n", member.Role))
		if member.Member != nil {
			sb.WriteString(fmt.Sprintf("Member Name: %s\n", member.Member.Name))
			sb.WriteString(fmt.Sprintf("Display Name: %s\n", member.Member.DisplayName))
			sb.WriteString(fmt.Sprintf("Type: %s\n", member.Member.Type))
		}
		sb.WriteString("\n")
	}

	if resp.NextPageToken != "" {
		sb.WriteString(fmt.Sprintf("Next Page Token: %s\n", resp.NextPageToken))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func getMemberHandler(ctx context.Context, request mcp.CallToolRequest, input GetMemberInput) (*mcp.CallToolResult, error) {
	service, err := services.ChatServiceFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat service: %v", err)
	}

	member, err := service.Spaces.Members.Get(input.MemberName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get member: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("Member Details:\n\n")
	sb.WriteString(fmt.Sprintf("Name: %s\n", member.Name))
	sb.WriteString(fmt.Sprintf("State: %s\n", member.State))
	sb.WriteString(fmt.Sprintf("Role: %s\n", member.Role))
	if member.Member != nil {
		sb.WriteString(fmt.Sprintf("Member Name: %s\n", member.Member.Name))
		sb.WriteString(fmt.Sprintf("Display Name: %s\n", member.Member.DisplayName))
		sb.WriteString(fmt.Sprintf("Type: %s\n", member.Member.Type))
		if member.Member.DomainId != "" {
			sb.WriteString(fmt.Sprintf("Domain ID: %s\n", member.Member.DomainId))
		}
	}
	sb.WriteString(fmt.Sprintf("Create Time: %s\n", member.CreateTime))

	return mcp.NewToolResultText(sb.String()), nil
}
