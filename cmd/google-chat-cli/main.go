package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/nguyenvanduocit/google-chat-mcp/services"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/chat/v1"
	"google.golang.org/api/option"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "list-spaces":
		runListSpaces(os.Args[2:])
	case "get-space":
		runGetSpace(os.Args[2:])
	case "list-messages":
		runListMessages(os.Args[2:])
	case "get-message":
		runGetMessage(os.Args[2:])
	case "send-message":
		runSendMessage(os.Args[2:])
	case "delete-message":
		runDeleteMessage(os.Args[2:])
	case "list-members":
		runListMembers(os.Args[2:])
	case "get-member":
		runGetMember(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: google-chat-cli <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  list-spaces      List all accessible Google Chat spaces")
	fmt.Println("  get-space        Get details of a specific space")
	fmt.Println("  list-messages    List messages in a space")
	fmt.Println("  get-message      Get a specific message")
	fmt.Println("  send-message     Send a message to a space")
	fmt.Println("  delete-message   Delete a message")
	fmt.Println("  list-members     List members of a space")
	fmt.Println("  get-member       Get details of a specific member")
	fmt.Println()
	fmt.Println("Global flags (available on every command):")
	fmt.Println("  --env string     Path to .env file (optional)")
	fmt.Println("  --output string  Output format: text (default) or json")
	fmt.Println()
	fmt.Println("Environment variables:")
	fmt.Println("  GOOGLE_CREDENTIALS_FILE   Path to OAuth2 credentials JSON")
	fmt.Println("  GOOGLE_TOKEN_FILE         Path to stored OAuth2 token JSON")
}

// commonFlags holds flags shared by all subcommands.
type commonFlags struct {
	env    string
	output string
}

func parseCommon(args []string, fs *flag.FlagSet) commonFlags {
	var cf commonFlags
	fs.StringVar(&cf.env, "env", "", "Path to .env file (optional)")
	fs.StringVar(&cf.output, "output", "text", "Output format: text or json")
	_ = fs.Parse(args)
	return cf
}

// buildChatServiceDirect loads credentials + token and returns an authenticated Chat service.
func buildChatServiceDirect(cf commonFlags) (*chat.Service, error) {
	if cf.env != "" {
		if err := godotenv.Load(cf.env); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load env file %s: %v\n", cf.env, err)
		}
	}

	credFile := os.Getenv("GOOGLE_CREDENTIALS_FILE")
	if credFile == "" {
		return nil, fmt.Errorf("GOOGLE_CREDENTIALS_FILE is not set")
	}

	tokenFile := os.Getenv("GOOGLE_TOKEN_FILE")
	if tokenFile == "" {
		dir := filepath.Dir(credFile)
		tokenFile = filepath.Join(dir, "google-token.json")
	}

	credBytes, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}

	cfg, err := google.ConfigFromJSON(credBytes, services.ListChatScopes()...)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	tokenBytes, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read token file %s: %w", tokenFile, err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(tokenBytes, &token); err != nil {
		return nil, fmt.Errorf("parse token file: %w", err)
	}

	httpClient := cfg.Client(context.Background(), &token)
	return chat.NewService(context.Background(), option.WithHTTPClient(httpClient))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fatal("encode json: %v", err)
	}
}

// --------------------------------------------------------------------------
// list-spaces
// --------------------------------------------------------------------------

func runListSpaces(args []string) {
	fs := flag.NewFlagSet("list-spaces", flag.ExitOnError)
	pageSize := fs.Int("page-size", 100, "Maximum number of spaces to return")
	pageToken := fs.String("page-token", "", "Page token for pagination")
	cf := parseCommon(args, fs)

	svc, err := buildChatServiceDirect(cf)
	if err != nil {
		fatal("%v", err)
	}

	call := svc.Spaces.List().PageSize(int64(*pageSize))
	if *pageToken != "" {
		call = call.PageToken(*pageToken)
	}

	resp, err := call.Context(context.Background()).Do()
	if err != nil {
		fatal("list spaces: %v", err)
	}

	if cf.output == "json" {
		outputJSON(resp)
		return
	}

	if len(resp.Spaces) == 0 {
		fmt.Println("No spaces found.")
		return
	}

	fmt.Printf("Found %d spaces:\n\n", len(resp.Spaces))
	for _, space := range resp.Spaces {
		fmt.Printf("Name: %s\n", space.Name)
		fmt.Printf("Display Name: %s\n", space.DisplayName)
		fmt.Printf("Type: %s\n", space.Type)
		if space.SpaceType != "" {
			fmt.Printf("Space Type: %s\n", space.SpaceType)
		}
		fmt.Println()
	}
	if resp.NextPageToken != "" {
		fmt.Printf("Next Page Token: %s\n", resp.NextPageToken)
	}
}

// --------------------------------------------------------------------------
// get-space
// --------------------------------------------------------------------------

func runGetSpace(args []string) {
	fs := flag.NewFlagSet("get-space", flag.ExitOnError)
	spaceName := fs.String("space-name", "", "Resource name of the space (e.g. spaces/AAAA1234)")
	cf := parseCommon(args, fs)

	if *spaceName == "" {
		fmt.Fprintln(os.Stderr, "error: --space-name is required")
		fs.Usage()
		os.Exit(1)
	}

	svc, err := buildChatServiceDirect(cf)
	if err != nil {
		fatal("%v", err)
	}

	space, err := svc.Spaces.Get(*spaceName).Context(context.Background()).Do()
	if err != nil {
		fatal("get space: %v", err)
	}

	if cf.output == "json" {
		outputJSON(space)
		return
	}

	fmt.Println("Space Details:")
	fmt.Println()
	fmt.Printf("Name: %s\n", space.Name)
	fmt.Printf("Display Name: %s\n", space.DisplayName)
	fmt.Printf("Type: %s\n", space.Type)
	if space.SpaceType != "" {
		fmt.Printf("Space Type: %s\n", space.SpaceType)
	}
	if space.SingleUserBotDm {
		fmt.Println("Single User Bot DM: true")
	}
	if space.Threaded {
		fmt.Println("Threaded: true")
	}
}

// --------------------------------------------------------------------------
// list-messages
// --------------------------------------------------------------------------

func runListMessages(args []string) {
	fs := flag.NewFlagSet("list-messages", flag.ExitOnError)
	spaceName := fs.String("space-name", "", "Resource name of the space (e.g. spaces/AAAA1234)")
	pageSize := fs.Int("page-size", 25, "Maximum number of messages to return")
	pageToken := fs.String("page-token", "", "Page token for pagination")
	cf := parseCommon(args, fs)

	if *spaceName == "" {
		fmt.Fprintln(os.Stderr, "error: --space-name is required")
		fs.Usage()
		os.Exit(1)
	}

	svc, err := buildChatServiceDirect(cf)
	if err != nil {
		fatal("%v", err)
	}

	call := svc.Spaces.Messages.List(*spaceName).
		PageSize(int64(*pageSize)).
		OrderBy("createTime desc")
	if *pageToken != "" {
		call = call.PageToken(*pageToken)
	}

	resp, err := call.Context(context.Background()).Do()
	if err != nil {
		fatal("list messages: %v", err)
	}

	if cf.output == "json" {
		outputJSON(resp)
		return
	}

	if len(resp.Messages) == 0 {
		fmt.Println("No messages found in this space.")
		return
	}

	fmt.Printf("Found %d messages:\n\n", len(resp.Messages))
	for _, msg := range resp.Messages {
		fmt.Print(formatMessage(msg))
		fmt.Println("---")
	}
	if resp.NextPageToken != "" {
		fmt.Printf("\nNext Page Token: %s\n", resp.NextPageToken)
	}
}

// --------------------------------------------------------------------------
// get-message
// --------------------------------------------------------------------------

func runGetMessage(args []string) {
	fs := flag.NewFlagSet("get-message", flag.ExitOnError)
	messageName := fs.String("message-name", "", "Resource name of the message (e.g. spaces/AAAA1234/messages/BBBB5678)")
	cf := parseCommon(args, fs)

	if *messageName == "" {
		fmt.Fprintln(os.Stderr, "error: --message-name is required")
		fs.Usage()
		os.Exit(1)
	}

	svc, err := buildChatServiceDirect(cf)
	if err != nil {
		fatal("%v", err)
	}

	msg, err := svc.Spaces.Messages.Get(*messageName).Context(context.Background()).Do()
	if err != nil {
		fatal("get message: %v", err)
	}

	if cf.output == "json" {
		outputJSON(msg)
		return
	}

	fmt.Print(formatMessage(msg))
}

// --------------------------------------------------------------------------
// send-message
// --------------------------------------------------------------------------

func runSendMessage(args []string) {
	fs := flag.NewFlagSet("send-message", flag.ExitOnError)
	spaceName := fs.String("space-name", "", "Resource name of the space (e.g. spaces/AAAA1234)")
	text := fs.String("text", "", "Text content of the message")
	threadKey := fs.String("thread-key", "", "Thread key for replying to a specific thread (optional)")
	cf := parseCommon(args, fs)

	if *spaceName == "" {
		fmt.Fprintln(os.Stderr, "error: --space-name is required")
		fs.Usage()
		os.Exit(1)
	}
	if *text == "" {
		fmt.Fprintln(os.Stderr, "error: --text is required")
		fs.Usage()
		os.Exit(1)
	}

	svc, err := buildChatServiceDirect(cf)
	if err != nil {
		fatal("%v", err)
	}

	message := &chat.Message{Text: *text}
	if *threadKey != "" {
		message.Thread = &chat.Thread{ThreadKey: *threadKey}
	}

	msg, err := svc.Spaces.Messages.Create(*spaceName, message).Context(context.Background()).Do()
	if err != nil {
		fatal("send message: %v", err)
	}

	if cf.output == "json" {
		outputJSON(msg)
		return
	}

	fmt.Println("Message sent successfully!")
	fmt.Println()
	fmt.Print(formatMessage(msg))
}

// --------------------------------------------------------------------------
// delete-message
// --------------------------------------------------------------------------

func runDeleteMessage(args []string) {
	fs := flag.NewFlagSet("delete-message", flag.ExitOnError)
	messageName := fs.String("message-name", "", "Resource name of the message to delete")
	cf := parseCommon(args, fs)

	if *messageName == "" {
		fmt.Fprintln(os.Stderr, "error: --message-name is required")
		fs.Usage()
		os.Exit(1)
	}

	svc, err := buildChatServiceDirect(cf)
	if err != nil {
		fatal("%v", err)
	}

	_, err = svc.Spaces.Messages.Delete(*messageName).Context(context.Background()).Do()
	if err != nil {
		fatal("delete message: %v", err)
	}

	if cf.output == "json" {
		outputJSON(map[string]string{"status": "deleted", "message": *messageName})
		return
	}

	fmt.Printf("Message %s deleted successfully!\n", *messageName)
}

// --------------------------------------------------------------------------
// list-members
// --------------------------------------------------------------------------

func runListMembers(args []string) {
	fs := flag.NewFlagSet("list-members", flag.ExitOnError)
	spaceName := fs.String("space-name", "", "Resource name of the space (e.g. spaces/AAAA1234)")
	pageSize := fs.Int("page-size", 100, "Maximum number of members to return")
	pageToken := fs.String("page-token", "", "Page token for pagination")
	cf := parseCommon(args, fs)

	if *spaceName == "" {
		fmt.Fprintln(os.Stderr, "error: --space-name is required")
		fs.Usage()
		os.Exit(1)
	}

	svc, err := buildChatServiceDirect(cf)
	if err != nil {
		fatal("%v", err)
	}

	call := svc.Spaces.Members.List(*spaceName).PageSize(int64(*pageSize))
	if *pageToken != "" {
		call = call.PageToken(*pageToken)
	}

	resp, err := call.Context(context.Background()).Do()
	if err != nil {
		fatal("list members: %v", err)
	}

	if cf.output == "json" {
		outputJSON(resp)
		return
	}

	if len(resp.Memberships) == 0 {
		fmt.Println("No members found in this space.")
		return
	}

	fmt.Printf("Found %d members:\n\n", len(resp.Memberships))
	for _, member := range resp.Memberships {
		fmt.Printf("Name: %s\n", member.Name)
		fmt.Printf("State: %s\n", member.State)
		fmt.Printf("Role: %s\n", member.Role)
		if member.Member != nil {
			fmt.Printf("Member Name: %s\n", member.Member.Name)
			fmt.Printf("Display Name: %s\n", member.Member.DisplayName)
			fmt.Printf("Type: %s\n", member.Member.Type)
		}
		fmt.Println()
	}
	if resp.NextPageToken != "" {
		fmt.Printf("Next Page Token: %s\n", resp.NextPageToken)
	}
}

// --------------------------------------------------------------------------
// get-member
// --------------------------------------------------------------------------

func runGetMember(args []string) {
	fs := flag.NewFlagSet("get-member", flag.ExitOnError)
	memberName := fs.String("member-name", "", "Resource name of the member (e.g. spaces/AAAA1234/members/CCCC9012)")
	cf := parseCommon(args, fs)

	if *memberName == "" {
		fmt.Fprintln(os.Stderr, "error: --member-name is required")
		fs.Usage()
		os.Exit(1)
	}

	svc, err := buildChatServiceDirect(cf)
	if err != nil {
		fatal("%v", err)
	}

	member, err := svc.Spaces.Members.Get(*memberName).Context(context.Background()).Do()
	if err != nil {
		fatal("get member: %v", err)
	}

	if cf.output == "json" {
		outputJSON(member)
		return
	}

	fmt.Println("Member Details:")
	fmt.Println()
	fmt.Printf("Name: %s\n", member.Name)
	fmt.Printf("State: %s\n", member.State)
	fmt.Printf("Role: %s\n", member.Role)
	if member.Member != nil {
		fmt.Printf("Member Name: %s\n", member.Member.Name)
		fmt.Printf("Display Name: %s\n", member.Member.DisplayName)
		fmt.Printf("Type: %s\n", member.Member.Type)
		if member.Member.DomainId != "" {
			fmt.Printf("Domain ID: %s\n", member.Member.DomainId)
		}
	}
	fmt.Printf("Create Time: %s\n", member.CreateTime)
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

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
