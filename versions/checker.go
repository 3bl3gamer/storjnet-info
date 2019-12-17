package versions

import (
	"net/http"
	"storjnet/core"
	"storjnet/utils"

	"github.com/ansel1/merry"
	"github.com/blang/semver"
	"github.com/go-pg/pg/v9"
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
		if err := utils.TGSendMessageMD(bot, chatID, text); err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}

func CheckVersions(tgBotToken, tgSocks5ProxyAddr string) error {
	db := utils.MakePGConnection()

	for _, cfg := range core.VersionConfigs {
		prevVersion := semver.Version{}
		_, err := db.QueryOne(&prevVersion,
			`SELECT version FROM versions WHERE kind = ? ORDER BY created_at DESC LIMIT 1`, cfg.Key)
		if err != nil && err != pg.ErrNoRows {
			return merry.Wrap(err)
		}

		curVersion, err := cfg.Version()
		if err != nil {
			return merry.Wrap(err)
		}
		log.Debug().Msgf("%s -> %s (%s)", prevVersion, curVersion, cfg.Key)

		if !curVersion.Equals(prevVersion) {
			text := cfg.MessageNew(curVersion)
			tgChatIDs, err := core.AppConfigInt64Slice(db, "tgbot:version_notif_ids", false)
			if err != nil {
				return merry.Wrap(err)
			}
			if err := sendTGMessages(tgBotToken, tgSocks5ProxyAddr, text, tgChatIDs); err != nil {
				return merry.Wrap(err)
			}
		}

		if !curVersion.Equals(prevVersion) {
			_, err := db.Exec(`INSERT INTO versions (kind, version) VALUES (?, ?)`, cfg.Key, curVersion)
			if err != nil {
				return merry.Wrap(err)
			}
		}
	}
	return nil
}
