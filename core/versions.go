package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ansel1/merry"
	"github.com/blang/semver"
	"github.com/rs/zerolog/log"
)

type prefixedVersion semver.Version

func (v *prefixedVersion) UnmarshalJSON(data []byte) (err error) {
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
	*v = prefixedVersion(sv)
	return nil
}

var GitHubOAuthToken = ""

var VersionConfigs = []struct {
	Key        string
	Version    func() (semver.Version, error)
	MessageNew func(semver.Version) string
	MessageCur func(semver.Version) string
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
		func(version semver.Version) string {
			return fmt.Sprintf("v%s (version.storj.io)", version)
		},
	},
	{
		"GitHub:latest",
		func() (semver.Version, error) {
			req, err := http.NewRequest("GET", "https://api.github.com/repos/storj/storj/releases/latest", nil)
			if err != nil {
				return semver.Version{}, merry.Wrap(err)
			}
			if GitHubOAuthToken != "" {
				req.Header.Set("Authorization", "token "+GitHubOAuthToken)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return semver.Version{}, merry.Wrap(err)
			}
			defer resp.Body.Close()
			params := &struct {
				Name    string
				TagName prefixedVersion `json:"tag_name"`
			}{}
			// if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
			// 	return semver.Version{}, merry.Wrap(err)
			// }
			buf, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return semver.Version{}, merry.Wrap(err)
			}
			if err := json.Unmarshal(buf, &params); err != nil {
				return semver.Version{}, merry.Wrap(err)
			}
			if semver.Version(params.TagName).Equals(semver.Version{}) {
				log.Warn().Str("resp", string(buf)).Msg("version is zero")
				return semver.Version{}, merry.New("version is zero")
			}
			return semver.Version(params.TagName), nil
		},
		func(version semver.Version) string {
			return fmt.Sprintf("Новый релиз *v%s*\nНа [ГитХабе](https://github.com/storj/storj/releases/tag/v%s), с ченджлогом и бинарниками.", version, version)
		},
		func(version semver.Version) string {
			return fmt.Sprintf("v%s ([GitHub](https://github.com/storj/storj/releases/tag/v%s))", version, version)
		},
	},
}
