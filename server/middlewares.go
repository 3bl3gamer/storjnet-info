package server

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"storj3stat/core"
	"strings"
	"sync"

	httputils "github.com/3bl3gamer/go-http-utils"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/julienschmidt/httprouter"
)

func withUserInner(handle httputils.HandlerExt, wr http.ResponseWriter, r *http.Request, ps httprouter.Params, mustBeLoggedIn bool) error {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	var user *core.User
	var err error

	// trying basic auth
	if username, password, ok := r.BasicAuth(); ok {
		user, err = core.FindUserByUsernameAndPassword(db, username, password)
		if err != nil && !merry.Is(err, core.ErrUserNotFound) {
			return merry.Wrap(err)
		}
	} else {
		// trying regular cookie sessid
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

var gzippers = sync.Pool{New: func() interface{} {
	gz, err := gzip.NewWriterLevel(nil, 2) // pings array: 1 - 62.9KB, 2 - 45.2KB, 3 - 45.0KB, 9 - 44.7KB
	if err != nil {
		panic(err)
	}
	return gz
}}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	return w.gz.Write(p)
}

func WithGzip(handle httputils.HandlerExt) httputils.HandlerExt {
	return func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			return handle(wr, r, ps)
		}
		wr.Header().Set("Content-Encoding", "gzip")
		gz := gzippers.Get().(*gzip.Writer)
		defer gzippers.Put(gz)
		gz.Reset(wr)
		err := handle(&gzipResponseWriter{wr, gz}, r, ps)
		if err != nil {
			return merry.Wrap(err)
		}
		return merry.Wrap(gz.Close())
	}
}
