package server

import (
	"context"
	"encoding/json"
	"net/http"
	"storj3stat/core"

	httputils "github.com/3bl3gamer/go-http-utils"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/julienschmidt/httprouter"
)

func withUserInner(handle httputils.HandlerExt, wr http.ResponseWriter, r *http.Request, ps httprouter.Params, mustBeLoggedIn bool) error {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)

	var user *core.User
	cookie, err := r.Cookie("sessid")
	if err == nil {
		sessid := cookie.Value
		user, err = core.FindUserBySessid(db, sessid)
		if err != nil && !merry.Is(err, core.ErrUserNotFound) {
			return merry.Wrap(err)
		}
		if user != nil {
			if err := core.UpdateSessionData(db, wr, user); err != nil {
				return merry.Wrap(err)
			}
		}
	}

	if mustBeLoggedIn && user == nil {
		wr.Header().Set("Content-Type", "application/json")
		return merry.Wrap(json.NewEncoder(wr).Encode(httputils.JsonError{Ok: false, Code: 403, Error: "FORBIDDEN"}))
	}

	r = r.WithContext(context.WithValue(r.Context(), CtxKeyUser, user))
	return handle(wr, r, ps)
}

func WithOptUser(handle httputils.HandlerExt) httputils.HandlerExt {
	return func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
		return withUserInner(handle, wr, r, ps, false)
	}
}

func WithUser(handle httputils.HandlerExt) httputils.HandlerExt {
	return func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
		return withUserInner(handle, wr, r, ps, true)
	}
}
