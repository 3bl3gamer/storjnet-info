package server

import (
	"context"
	"fmt"
	"net/http"
	"storj3stat/core"
	"storj3stat/utils"
	"time"

	httputils "github.com/3bl3gamer/go-http-utils"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/julienschmidt/httprouter"
	"storj.io/storj/pkg/storj"
)

func HandleIndex(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	return map[string]interface{}{"FPath": "index.html"}, nil
}

func HandlePingMyNode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	return map[string]interface{}{"FPath": "ping_my_node.html"}, nil
}

func HandleUserDashboard(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	user := r.Context().Value(CtxKeyUser).(*core.User)
	if user == nil {
		return map[string]interface{}{"FPath": "user_dashboard.html", "User": user}, nil
	}
	nodes, err := core.LoadUserNodes(db, user)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return map[string]interface{}{"FPath": "user_dashboard.html", "User": user, "UserNodes": nodes}, nil
}

func HandleLang(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	if err := r.ParseForm(); err != nil {
		return merry.Wrap(err)
	}
	if lang := r.Form.Get("lang"); lang != "" {
		cookie := &http.Cookie{
			Name:    "lang",
			Value:   lang,
			Path:    "/",
			Expires: time.Now().Add(365 * time.Hour),
		}
		wr.Header().Add("Set-Cookie", cookie.String())
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(wr, r, ref, 303)
	return nil
}

func HandleAPIPingMyNode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	sat := r.Context().Value(CtxKeySatellite).(*utils.Satellite)
	params := &struct {
		ID, Address string
		DialOnly    bool
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}

	id, err := storj.NodeIDFromString(params.ID)
	if err != nil {
		return httputils.JsonError{Code: 400, Error: "NODE_ID_DECODE_ERROR", Description: err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stt := time.Now()
	conn, err := sat.Dial(ctx, params.Address, id)
	if err != nil {
		return httputils.JsonError{Code: 400, Error: "NODE_DIAL_ERROR", Description: err.Error()}, nil
	}
	dialDuration := time.Now().Sub(stt).Seconds()
	defer conn.Close()

	var pingDuration float64
	if !params.DialOnly {
		stt := time.Now()
		if err := sat.Ping(ctx, conn); err != nil {
			return httputils.JsonError{Code: 400, Error: "NODE_PING_ERROR", Description: err.Error()}, nil
		}
		pingDuration = time.Now().Sub(stt).Seconds()
	}
	return map[string]interface{}{"pingDuration": pingDuration, "dialDuration": dialDuration}, nil
}

func HandleAPIRegister(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	params := &struct {
		Email, Password string
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}
	if len(params.Email) < 3 {
		return httputils.JsonError{Code: 400, Error: "WRONG_EMAIL"}, nil
	}
	_, err := core.RegisterUser(db, wr, params.Email, params.Password)
	if merry.Is(err, core.ErrEmailExsists) {
		return httputils.JsonError{Code: 400, Error: "EMAIL_EXISTS"}, nil
	}
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return "ok", nil
}

func HandleAPILogin(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	params := &struct {
		Email, Password string
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}
	_, err := core.LoginUser(db, wr, params.Email, params.Password)
	if merry.Is(err, core.ErrUserNotFound) {
		return httputils.JsonError{Code: 403, Error: "WRONG_EMAIL_OR_PASSWORD"}, nil
	}
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return "ok", nil
}

func HandleAPISetUserNode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	user := r.Context().Value(CtxKeyUser).(*core.User)
	node := &core.Node{}
	if jsonErr := unmarshalNodeFromBody(r, node); jsonErr != nil {
		return *jsonErr, nil
	}
	err := core.SetUserNode(db, user, node)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	fmt.Println(node)
	return node, nil
}

func HandleAPIDelUserNode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	user := r.Context().Value(CtxKeyUser).(*core.User)
	params := &struct {
		ID storj.NodeID
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}
	err := core.DelUserNode(db, user, params.ID)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return "ok", nil
}

func HandleHtml500(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	tmplHnd := r.Context().Value(httputils.CtxKeyMain).(*httputils.MainCtx).TemplateHandler
	return merry.Wrap(tmplHnd.RenderTemplate(wr, r, map[string]interface{}{"FPath": "500.html"}))
}
