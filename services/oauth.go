package services

import (
	"google.golang.org/api/chat/v1"
)

// ListChatScopes returns all Google Chat API scopes.
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
