package gm

import (
	"autoGrocery/internal/token"
	"context"
	"encoding/base64"
	"log"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const User = "me"

type Manager struct {
	Service *gmail.Service
}

func GetGmailManager(ctx context.Context, config *oauth2.Config, tokFile string) Manager {
	service, err := GetGmailService(ctx, config, tokFile)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}
	return Manager{Service: service}
}

func GetGmailService(ctx context.Context, config *oauth2.Config, tokFile string) (*gmail.Service, error) {
	client, errGetClient := token.GetClient(config, tokFile)
	if errGetClient != nil {
		return nil, errGetClient
	}
	return gmail.NewService(ctx, option.WithHTTPClient(client))
}

func GetMessagesFromLabel(manager Manager, labelId string) []*gmail.Message {
	messages := make([]*gmail.Message, 0)
	mr, err := manager.Service.Users.Messages.List(User).LabelIds(labelId).Do()
	if err != nil {
		slog.Error("Unable to fetch messages from gmail", "ERROR", err)
	}
	if len(mr.Messages) == 0 {
		slog.Debug("No messages found.")
		return messages
	}
	for _, l := range mr.Messages {
		m, _ := manager.Service.Users.Messages.Get(User, l.Id).Do()
		messages = append(messages, m)
	}
	return messages
}

func GetAttachmentFromMessage(manager Manager, messageId string, attachmentId string) []byte {
	attachment, err := manager.Service.Users.Messages.Attachments.Get(User, messageId, attachmentId).Do()
	if err != nil {
		slog.Error("Unable to retrieve attachment from message", "ERROR", err)
	}
	decodedFile, err := base64.URLEncoding.DecodeString(attachment.Data)
	if err != nil {
		slog.Error("Unable to decode attachment from email", "ERROR", err)
	}
	return decodedFile
}

func DeleteMessage(manager Manager, messageId string) {
	err := manager.Service.Users.Messages.Delete(User, messageId).Do()
	if err != nil {
		slog.Error("Unable to delete email", "MESSAGE_ID", messageId, "ERROR", err)
	}
}
