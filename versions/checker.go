package versions

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"storj3stat/utils"

	"github.com/ansel1/merry"
	"github.com/blang/semver"
	"github.com/go-pg/pg/v9"
	"golang.org/x/net/proxy"
)

type PrefixedVersion semver.Version

type Versions struct {
	StorjIo semver.Version
	GitHub  semver.Version
}

func (v *PrefixedVersion) UnmarshalJSON(data []byte) (err error) {
	var versionString string
	if err = json.Unmarshal(data, &versionString); err != nil {
		return merry.Wrap(err)
	}
	if versionString[0] == 'v' {
		versionString = versionString[1:]
	}
	sv, err := semver.Parse(versionString)
	if err != nil {
		return merry.Wrap(err)
	}
	*v = PrefixedVersion(sv)
	return nil
}

var versionConfigs = []struct {
	key         string
	getFunc     func() (semver.Version, error)
	messageFunc func(semver.Version) string
}{
	{
		"StorjIo:storagenode",
		func() (semver.Version, error) {
			resp, err := http.Get("https://version.storj.io/")
			if err != nil {
				return semver.Version{}, merry.Wrap(err)
			}
			defer resp.Body.Close()
			params := &struct {
				Processes struct {
					Storagenode struct {
						Suggested struct{ Version semver.Version }
					}
				}
			}{}
			if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
				return semver.Version{}, merry.Wrap(err)
			}
			return params.Processes.Storagenode.Suggested.Version, nil
		},
		func(version semver.Version) string {
			return fmt.Sprintf("Новая версия *v%s*\nРекомендуемая для нод на version.storj.io", version)
		},
	},
	{
		"GitHub:latest",
		func() (semver.Version, error) {
			resp, err := http.Get("https://api.github.com/repos/storj/storj/releases/latest")
			if err != nil {
				return semver.Version{}, merry.Wrap(err)
			}
			defer resp.Body.Close()
			params := &struct {
				Name    string
				TagName PrefixedVersion `json:"tag_name"`
			}{}
			if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
				return semver.Version{}, merry.Wrap(err)
			}
			return semver.Version(params.TagName), nil
		},
		func(version semver.Version) string {
			return fmt.Sprintf("Новый релиз *v%s*\nНа [ГитХабе](https://github.com/storj/storj/releases/tag/v%s), с ченджлогом и бинарниками.", version, version)
		},
	},
}

func sendTGMessage(httpClient *http.Client, botID, text, chatID string) error {
	query := make(url.Values)
	query.Set("chat_id", chatID)
	query.Set("text", text)
	query.Set("parse_mode", "Markdown")
	query.Set("disable_web_page_preview", "1")
	res, err := httpClient.Get("https://api.telegram.org/bot" + botID + "/sendMessage?" + query.Encode())
	if err != nil {
		return merry.Wrap(err)
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return merry.Wrap(err)
	}
	tgRes := struct{ Ok bool }{}
	if err := json.Unmarshal(buf, &tgRes); err != nil {
		return merry.Wrap(err)
	}
	if !tgRes.Ok {
		return merry.New("wrong TG response: " + string(buf))
	}
	return nil
}

func sendTGMessages(botID, text string, chatIDs []string) error {
	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:9050", nil, proxy.Direct)
	if err != nil {
		return merry.Wrap(err)
	}
	httpTransport := &http.Transport{Dial: dialer.Dial}
	httpClient := &http.Client{Transport: httpTransport}

	for _, chatID := range chatIDs {
		if err := sendTGMessage(httpClient, botID, text, chatID); err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}

func CheckVersions(tgBotToken string) error {
	tgChatIDs := []string{"78542303"}
	db := utils.MakePGConnection()

	for _, cfg := range versionConfigs {
		prevVersion := semver.Version{}
		_, err := db.QueryOne(&prevVersion, `SELECT version FROM versions WHERE kind = ? ORDER BY created_at DESC LIMIT 1`, cfg.key)
		if err != nil && err != pg.ErrNoRows {
			return merry.Wrap(err)
		}

		curVersion, err := cfg.getFunc()
		if err != nil {
			return merry.Wrap(err)
		}
		log.Printf("%s -> %s (%s)", prevVersion, curVersion, cfg.key)

		if !curVersion.Equals(prevVersion) {
			text := cfg.messageFunc(curVersion)
			if err := sendTGMessages(tgBotToken, text, tgChatIDs); err != nil {
				return merry.Wrap(err)
			}
		}

		if !curVersion.Equals(prevVersion) {
			_, err := db.Exec(`INSERT INTO versions (kind, version) VALUES (?, ?)`, cfg.key, curVersion)
			if err != nil {
				return merry.Wrap(err)
			}
		}
	}
	return nil
}
