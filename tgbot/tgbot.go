package tgbot

import (
	"context"
	"net/http"
	"net/url"
	"storjnet/core"
	"storjnet/utils"
	"strconv"
	"strings"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v10"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
)

var ErrActionNotAllowed = merry.New("action not allowed")

type cmdHandler func(*tgbotapi.BotAPI, *pg.DB, tgbotapi.Update, string) error

type WebhookConfig struct {
	URL        string
	ListenAddr string
	ListenPath string
}

func isReplyToBotInGroupChat(bot *tgbotapi.BotAPI, update tgbotapi.Update) bool {
	msg := update.Message
	if msg == nil || msg.From == nil || msg.ReplyToMessage == nil || msg.ReplyToMessage.From == nil {
		return false
	}
	return int64(msg.From.ID) != msg.Chat.ID && msg.ReplyToMessage.From.ID == bot.Self.ID
}

func justSend(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	return merry.Wrap(utils.TGSendMessageMD(bot, chatID, text))
}

func extractCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) (string, string) {
	m := update.Message
	if m == nil {
		return "", ""
	}
	if m.Entities == nil || len(*m.Entities) == 0 {
		return "", ""
	}
	entity := (*m.Entities)[0]
	if entity.Offset > 0 || entity.Type != "bot_command" {
		return "", ""
	}
	cmdText := m.Text[0:entity.Length]
	usernameSuffix := "@" + bot.Self.UserName
	if strings.HasSuffix(cmdText, usernameSuffix) {
		return cmdText[:len(cmdText)-len(usernameSuffix)], strings.TrimSpace(m.Text[entity.Offset:])
	}
	return cmdText, strings.TrimSpace(m.Text[entity.Offset:])
}

func extractSubscriptorID(bot *tgbotapi.BotAPI, db *pg.DB, message *tgbotapi.Message) (int64, error) {
	from := message.From
	chat := message.Chat

	if chat.IsPrivate() {
		return int64(from.ID), nil
	} else if chat.IsGroup() || chat.IsSuperGroup() {
		admins, err := bot.GetChatAdministrators(chat.ChatConfig())
		if err != nil {
			return 0, merry.Wrap(err)
		}
		for _, admin := range admins {
			if admin.User.ID == from.ID {
				return chat.ID, nil
			}
		}
		return 0, ErrActionNotAllowed.Here()
	}
	return 0, merry.Errorf("unexpected chat type: %s in %#v", chat.Type, chat)
}

func sendAction(bot *tgbotapi.BotAPI, chatID int64, action string) error {
	v := url.Values{}
	v.Add("chat_id", strconv.FormatInt(chatID, 10))
	v.Add("action", action)
	_, err := bot.MakeRequest("sendChatAction", v)
	return merry.Wrap(err)
}

func subscripbe(db *pg.DB, id int64) error {
	log.Debug().Int64("id", id).Msg("subscribing")
	err := db.RunInTransaction(context.Background(), func(tx *pg.Tx) error {
		ids, err := core.AppConfigInt64Slice(tx, "tgbot:version_notif_ids", true)
		if err != nil {
			return merry.Wrap(err)
		}
		for _, curID := range ids {
			if curID == id {
				return nil
			}
		}
		return merry.Wrap(core.AppConfigSet(tx, "tgbot:version_notif_ids", append(ids, id)))
	})
	return merry.Wrap(err)
}

func unsubscripbe(db *pg.DB, id int64) error {
	log.Debug().Int64("id", id).Msg("unsubscribing")
	err := db.RunInTransaction(context.Background(), func(tx *pg.Tx) error {
		ids, err := core.AppConfigInt64Slice(tx, "tgbot:version_notif_ids", true)
		if err != nil {
			return merry.Wrap(err)
		}
		for i, curID := range ids {
			if curID == id {
				newIds := append(ids[0:i], ids[i+1:]...)
				return merry.Wrap(core.AppConfigSet(tx, "tgbot:version_notif_ids", newIds))
			}
		}
		return nil
	})
	return merry.Wrap(err)
}

func handleStart(bot *tgbotapi.BotAPI, db *pg.DB, update tgbotapi.Update, args string) error {
	return justSend(bot, update.Message.Chat.ID, "Привет.")
}

func handleVersions(bot *tgbotapi.BotAPI, db *pg.DB, update tgbotapi.Update, args string) error {
	sendAction(bot, update.Message.Chat.ID, "typing")

	text := ""
	for _, checker := range core.MakeCurVersionCheckers() {
		err := checker.FetchCurVersion()
		if err == nil {
			text += checker.MessageCur() + "\n"
		} else {
			text += checker.Key() + ": " + err.Error() + "\n"
		}

	}
	_, err := bot.Send(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:           update.Message.Chat.ID,
			ReplyToMessageID: 0,
		},
		Text:                  text,
		DisableWebPagePreview: true,
		ParseMode:             "Markdown",
	})
	return merry.Wrap(err)
}

func handleSubscribe(bot *tgbotapi.BotAPI, db *pg.DB, update tgbotapi.Update, args string) error {
	id, err := extractSubscriptorID(bot, db, update.Message)
	if merry.Is(err, ErrActionNotAllowed) {
		return merry.Wrap(justSend(bot, update.Message.Chat.ID, "У тебя здесь нет власти!\nДля включения персональных уведомлений пиши в ЛС."))
	}
	if err != nil {
		return merry.Wrap(err)
	}
	if err := subscripbe(db, id); err != nil {
		return merry.Wrap(err)
	}
	return merry.Wrap(justSend(bot, update.Message.Chat.ID, "Буду присылать сюда уведомления о версиях."))
}

func handleUnsubscribe(bot *tgbotapi.BotAPI, db *pg.DB, update tgbotapi.Update, args string) error {
	id, err := extractSubscriptorID(bot, db, update.Message)
	if merry.Is(err, ErrActionNotAllowed) {
		return merry.Wrap(justSend(bot, update.Message.Chat.ID, "У тебя здесь нет власти!"))
	}
	if err != nil {
		return merry.Wrap(err)
	}
	if err := unsubscripbe(db, id); err != nil {
		return merry.Wrap(err)
	}
	return merry.Wrap(justSend(bot, update.Message.Chat.ID, "Отключил уведомления о версиях."))
}

func StartTGBot(tgBotToken, socks5ProxyAddr string, webhook *WebhookConfig) error {
	db := utils.MakePGConnection()

	httpClient := &http.Client{}
	if socks5ProxyAddr != "" {
		// auth := &proxy.Auth{User: *socksUser, Password: *socksPassword}
		dialer, err := proxy.SOCKS5("tcp", socks5ProxyAddr, nil, proxy.Direct)
		if err != nil {
			return merry.Wrap(err)
		}
		httpTransport := &http.Transport{Dial: dialer.Dial}
		httpClient.Transport = httpTransport
	}

	bot, err := tgbotapi.NewBotAPIWithClient(tgBotToken, httpClient)
	if err != nil {
		return merry.Wrap(err)
	}

	log.Info().Str("username", bot.Self.UserName).Str("name", bot.Self.FirstName).Msg("authorized")

	var updates tgbotapi.UpdatesChannel
	if webhook != nil {
		log.Info().Msg("using webhook")
		_, err = bot.SetWebhook(tgbotapi.NewWebhook(webhook.URL))
		if err != nil {
			return merry.Wrap(err)
		}
		updates = bot.ListenForWebhook(webhook.ListenPath)
		go func() { log.Fatal().Err(http.ListenAndServe(webhook.ListenAddr, nil)) }()
	} else {
		log.Info().Msg("using polling")
		if _, err := bot.RemoveWebhook(); err != nil {
			return merry.Wrap(err)
		}
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates, err = bot.GetUpdatesChan(u)
		if err != nil {
			return merry.Wrap(err)
		}
	}

	handlers := map[string]cmdHandler{
		"/start":       handleStart,
		"/versions":    handleVersions,
		"/version":     handleVersions,
		"/winver":      handleVersions,
		"/subscribe":   handleSubscribe,
		"/unsubscribe": handleUnsubscribe,
	}

	for update := range updates {
		// buf, _ := json.Marshal(update)
		// fmt.Println(">>> " + string(buf))
		cmd, args := extractCommand(bot, update)
		if handler, ok := handlers[cmd]; ok {
			if err := handler(bot, db, update, args); err != nil {
				return merry.Wrap(err)
			}
		} else if update.Message != nil && update.Message.Text != "" && !isReplyToBotInGroupChat(bot, update) {
			justSend(bot, update.Message.Chat.ID, "Не понял.")
		}
	}
	return nil
}
