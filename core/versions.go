package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ansel1/merry"
	"github.com/blang/semver"
	"github.com/go-pg/pg/v9"
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

type VersionChecker interface {
	Key() string
	FetchPrevVersion() error
	FetchCurVersion() error
	VersionHasChanged() bool
	SaveCurVersion() error
	Versions() (string, string)
	MessageNew() string
	MessageCur() string
}

type CurVersionChecker interface {
	Key() string
	FetchCurVersion() error
	MessageCur() string
}

type StrojIoVersionChecker struct {
	db          *pg.DB
	prevVersion semver.Version
	curVersion  semver.Version
}

func (c *StrojIoVersionChecker) Key() string {
	return "StorjIo:storagenode"
}

func (c *StrojIoVersionChecker) FetchPrevVersion() error {
	_, err := c.db.QueryOne(&c.prevVersion,
		`SELECT version FROM versions WHERE kind = ? ORDER BY created_at DESC LIMIT 1`, c.Key())
	if err != nil && err != pg.ErrNoRows {
		return merry.Wrap(err)
	}
	return nil
}

func (c *StrojIoVersionChecker) SaveCurVersion() error {
	_, err := c.db.Exec(`INSERT INTO versions (kind, version) VALUES (?, ?)`, c.Key(), c.curVersion)
	return merry.Wrap(err)
}

func (c *StrojIoVersionChecker) FetchCurVersion() error {
	resp, err := http.Get("https://version.storj.io/")
	if err != nil {
		return merry.Wrap(err)
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
		return merry.Wrap(err)
	}
	c.curVersion = params.Processes.Storagenode.Suggested.Version
	return nil
}

func (c *StrojIoVersionChecker) VersionHasChanged() bool {
	return !c.curVersion.Equals(c.prevVersion)
}

func (c *StrojIoVersionChecker) Versions() (string, string) {
	return c.prevVersion.String(), c.curVersion.String()
}

func (c *StrojIoVersionChecker) MessageNew() string {
	return fmt.Sprintf("Новая версия *v%s*\nРекомендуемая для нод на version.storj.io", c.curVersion)
}

func (c *StrojIoVersionChecker) MessageCur() string {
	return fmt.Sprintf("v%s (version.storj.io)", c.curVersion)
}

type GitHubVersionChecker struct {
	db          *pg.DB
	prevVersion semver.Version
	curVersion  semver.Version
}

func (c *GitHubVersionChecker) Key() string {
	return "GitHub:latest"
}

func (c *GitHubVersionChecker) FetchPrevVersion() error {
	_, err := c.db.QueryOne(&c.prevVersion,
		`SELECT version FROM versions WHERE kind = ? ORDER BY created_at DESC LIMIT 1`, c.Key())
	if err != nil && err != pg.ErrNoRows {
		return merry.Wrap(err)
	}
	return nil
}

func (c *GitHubVersionChecker) SaveCurVersion() error {
	_, err := c.db.Exec(`INSERT INTO versions (kind, version) VALUES (?, ?)`, c.Key(), c.curVersion)
	return merry.Wrap(err)
}

func (c *GitHubVersionChecker) FetchCurVersion() error {
	req, err := http.NewRequest("GET", "https://api.github.com/repos/storj/storj/releases/latest", nil)
	if err != nil {
		return merry.Wrap(err)
	}
	if GitHubOAuthToken != "" {
		req.Header.Set("Authorization", "token "+GitHubOAuthToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return merry.Wrap(err)
	}
	defer resp.Body.Close()
	params := &struct {
		Name    string
		TagName prefixedVersion `json:"tag_name"`
	}{}
	// if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
	// 	return merry.Wrap(err)
	// }
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return merry.Wrap(err)
	}
	if err := json.Unmarshal(buf, &params); err != nil {
		return merry.Wrap(err)
	}
	if semver.Version(params.TagName).Equals(semver.Version{}) {
		log.Warn().Str("resp", string(buf)).Msg("version is zero")
		return merry.New("version is zero")
	}
	c.curVersion = semver.Version(params.TagName)
	return nil
}

func (c *GitHubVersionChecker) VersionHasChanged() bool {
	return !c.curVersion.Equals(c.prevVersion)
}

func (c *GitHubVersionChecker) Versions() (string, string) {
	return c.prevVersion.String(), c.curVersion.String()
}

func (c *GitHubVersionChecker) MessageNew() string {
	return fmt.Sprintf("Новый релиз *v%s*\nНа [ГитХабе](https://github.com/storj/storj/releases/tag/v%s), с ченджлогом и бинарниками.", c.curVersion, c.curVersion)
}

func (c *GitHubVersionChecker) MessageCur() string {
	return fmt.Sprintf("v%s ([GitHub](https://github.com/storj/storj/releases/tag/v%s))", c.curVersion, c.curVersion)
}

func MakeCurVersionCheckers() []CurVersionChecker {
	return []CurVersionChecker{&StrojIoVersionChecker{}, &GitHubVersionChecker{}}
}

func MakeVersionCheckers(db *pg.DB) []VersionChecker {
	return []VersionChecker{&StrojIoVersionChecker{db: db}, &GitHubVersionChecker{db: db}}
}
