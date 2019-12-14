package utils

import (
	"github.com/ansel1/merry"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func TGSendMessageMD(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	_, err := bot.Send(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:           chatID,
			ReplyToMessageID: 0,
		},
		Text:                  text,
		DisableWebPagePreview: true,
		ParseMode:             "Markdown",
	})
	return merry.Wrap(err)
}
