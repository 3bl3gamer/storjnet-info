package core

import (
	"net/http"
	"storjnet/utils"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
)

const SessionDuration = 365 * 24 * time.Hour

// var ErrEmailExsists = merry.New("email_exists")
var ErrUsernameExsists = merry.New("username_exists")
var ErrUserNotFound = merry.New("user_not_found")

type User struct {
	ID           int64
	Email        string
	Username     string
	PasswordHash string
	Sessid       string
	CreatedAt    time.Time
	LastSeenAt   time.Time
}

func RegisterUser(db *pg.DB, wr http.ResponseWriter, username, password string) (*User, error) {
	user := &User{}
	_, err := db.QueryOne(user,
		"INSERT INTO users (username, password_hash, sessid) VALUES (?, crypt(?, gen_salt('bf')), gen_random_uuid()) RETURNING *",
		username, password)
	if utils.IsConstrError(err, "users", "unique_violation", "users_username_key") {
		return nil, ErrUsernameExsists.Here()
	}
	if err != nil {
		return nil, merry.Wrap(err)
	}
	setSessionCookie(wr, user.Sessid)
	return user, nil
}

func LoginUser(db *pg.DB, wr http.ResponseWriter, username, password string) (*User, error) {
	user, err := FindUserByUsernameAndPassword(db, username, password)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	setSessionCookie(wr, user.Sessid)
	UpdateUserLastSeenAtIfNeed(db, user)
	return user, nil
}

func FindUserBySessid(db *pg.DB, sessid string) (*User, error) {
	user := &User{}
	err := db.Model(user).Where("sessid = ?", sessid).Select()
	if err == pg.ErrNoRows {
		return nil, ErrUserNotFound.Here()
	}
	if perr, ok := merry.Unwrap(err).(pg.Error); ok {
		if strings.HasPrefix(perr.Field('M'), "invalid input syntax for type uuid:") {
			return nil, ErrUserNotFound.Here()
		}
	}
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return user, nil
}

func FindUserByUsernameAndPassword(db *pg.DB, username, password string) (*User, error) {
	user := &User{}
	err := db.Model(user).Where("username = ? AND password_hash = crypt(?, password_hash)", username, password).Select()
	if err == pg.ErrNoRows {
		return nil, ErrUserNotFound.Here()
	}
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return user, nil
}

func UpdateSessionData(db *pg.DB, wr http.ResponseWriter, user *User) error {
	setSessionCookie(wr, user.Sessid)
	return nil
}

func setSessionCookie(wr http.ResponseWriter, sessid string) {
	cookie := &http.Cookie{
		Name:     "sessid",
		Value:    sessid,
		Path:     "/",
		Expires:  time.Now().Add(SessionDuration),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	wr.Header().Set("Set-Cookie", cookie.String())
}

func UpdateUserLastSeenAtIfNeed(db *pg.DB, user *User) {
	if time.Since(user.LastSeenAt) < time.Minute {
		return
	}
	id := user.ID
	go func() {
		_, err := db.Exec("UPDATE users SET last_seen_at = now() WHERE id = ?", id)
		if err != nil {
			log.Error().Err(err).Int64("user_id", id).Msg("failed to update last_seen_at")
		}
	}()
}
