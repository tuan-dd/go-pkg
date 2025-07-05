package thirdPkg

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type SlackConnection struct {
	conn    *tgbotapi.BotAPI
	channel string
}

type FormatMsg interface {
	format() string
}

func NewSlackConnection(token, channel string) (*SlackConnection, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}
	return &SlackConnection{
		conn:    bot,
		channel: channel,
	}, nil
}

func (s *SlackConnection) SendMsg(message FormatMsg) error {
	msg := tgbotapi.NewMessageToChannel(s.channel, message.format())
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := s.conn.Send(msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}
