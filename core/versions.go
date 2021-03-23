package core

import (
	"encoding/json"
	"fmt"
	"io"
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
	DebugVersions() (string, string)
	MessageNew() string
	MessageCur() string
}

type CurVersionChecker interface {
	Key() string
	FetchCurVersion() error
	MessageCur() string
}

type Cursor string

func (c Cursor) String() string {
	if len(c) > 7 {
		c = c[:3] + "…" + c[len(c)-3:]
	}
	return string(c)
}

func (c Cursor) IsFinal() bool {
	for _, char := range c {
		if char != 'f' {
			return false
		}
	}
	return len(c) > 0
}

type VersionWithCursor struct {
	Version semver.Version
	Cursor  Cursor
}

func (v VersionWithCursor) VString() string {
	return fmt.Sprintf("v%s, cursor: %s", v.Version, v.Cursor)
}

type StrojIoVersionChecker struct {
	db          *pg.DB
	prevVersion VersionWithCursor
	curVersion  VersionWithCursor
}

func (c *StrojIoVersionChecker) Key() string {
	return "StorjIo:storagenode"
}

func (c *StrojIoVersionChecker) FetchPrevVersion() error {
	_, err := c.db.QueryOne(&c.prevVersion,
		`SELECT version, extra->>'cursor' AS cursor FROM versions WHERE kind = ? ORDER BY created_at DESC LIMIT 1`,
		c.Key())
	if err != nil && err != pg.ErrNoRows {
		return merry.Wrap(err)
	}
	return nil
}

func (c *StrojIoVersionChecker) SaveCurVersion() error {
	_, err := c.db.Exec(`INSERT INTO versions (kind, version, extra) VALUES (?, ?, json_build_object('cursor', ?))`,
		c.Key(), c.curVersion.Version, string(c.curVersion.Cursor))
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
				Rollout   struct{ Cursor string }
			}
		}
	}{}
	if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
		return merry.Wrap(err)
	}
	c.curVersion.Version = params.Processes.Storagenode.Suggested.Version
	c.curVersion.Cursor = Cursor(params.Processes.Storagenode.Rollout.Cursor)
	return nil
}

func (c *StrojIoVersionChecker) VersionHasChanged() bool {
	return !c.curVersion.Version.Equals(c.prevVersion.Version) ||
		(c.curVersion.Cursor != c.prevVersion.Cursor && c.curVersion.Cursor.IsFinal())
}

func (c *StrojIoVersionChecker) DebugVersions() (string, string) {
	return c.prevVersion.VString(), c.curVersion.VString()
}

func (c *StrojIoVersionChecker) MessageNew() string {
	if c.curVersion.Version.Equals(c.prevVersion.Version) && c.curVersion.Cursor.IsFinal() {
		return fmt.Sprintf("Финальный курсор *%s* (v%s) на version.storj.io",
			c.curVersion.Cursor, c.curVersion.Version)
	}
	return fmt.Sprintf("Новая версия *v%s* (cursor: %s)\nРекомендуемая для нод на version.storj.io",
		c.curVersion.Version, c.curVersion.Cursor)
}

func (c *StrojIoVersionChecker) MessageCur() string {
	return fmt.Sprintf("%s (version.storj.io)", c.curVersion.VString())
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
	buf, err := io.ReadAll(resp.Body)
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

func (c *GitHubVersionChecker) DebugVersions() (string, string) {
	return "v" + c.prevVersion.String(), "v" + c.curVersion.String()
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
