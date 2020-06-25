package versions

import (
	"net/http"
	"storjnet/core"
	"storjnet/utils"
	"time"

	"github.com/ansel1/merry"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
)

func sendTGMessages(botToken, socks5ProxyAddr, text string, chatIDs []int64) error {
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

	bot, err := tgbotapi.NewBotAPIWithClient(botToken, httpClient)
	if err != nil {
		return merry.Wrap(err)
	}

	for _, chatID := range chatIDs {
		for i := 0; i < 3; i++ {
			err := utils.TGSendMessageMD(bot, chatID, text)
			if err == nil {
				break
			} else {
				log.Error().Err(err).Int("iter", i).Int64("chatID", chatID).Msg("message sending error")
				time.Sleep(time.Second)
			}
		}
	}
	return nil
}

func CheckVersions(tgBotToken, tgSocks5ProxyAddr string) error {
	db := utils.MakePGConnection()

	for _, checker := range core.MakeVersionCheckers(db) {
		if err := checker.FetchPrevVersion(); err != nil {
			return merry.Wrap(err)
		}
		if err := checker.FetchCurVersion(); err != nil {
			return merry.Wrap(err)
		}
		prevVersion, curVersion := checker.DebugVersions()
		log.Debug().Msgf("%s -> %s (%s)", prevVersion, curVersion, checker.Key())

		if checker.VersionHasChanged() {
			text := checker.MessageNew()
			tgChatIDs, err := core.AppConfigInt64Slice(db, "tgbot:version_notif_ids", false)
			if err != nil {
				return merry.Wrap(err)
			}
			if err := sendTGMessages(tgBotToken, tgSocks5ProxyAddr, text, tgChatIDs); err != nil {
				return merry.Wrap(err)
			}
			if err := checker.SaveCurVersion(); err != nil {
				return merry.Wrap(err)
			}
		}
	}
	return nil
}
