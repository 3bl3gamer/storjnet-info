package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"storjnet/core"
	"storjnet/utils"
	"strings"
	"time"

	httputils "github.com/3bl3gamer/go-http-utils"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/julienschmidt/httprouter"
	"storj.io/storj/pkg/storj"
)

func HandleIndex(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	user := r.Context().Value(CtxKeyUser).(*core.User)
	nodes, err := core.LoadSatNodes(db)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return map[string]interface{}{"FPath": "index.html", "User": user, "SatNodes": nodes}, nil
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
	var userText string
	_, err = db.QueryOne(&userText, `SELECT text FROM user_texts WHERE user_id = ? ORDER BY updated_at DESC LIMIT 1`, user.ID)
	if err != nil && err != pg.ErrNoRows {
		return nil, merry.Wrap(err)
	}
	return map[string]interface{}{"FPath": "user_dashboard.html", "User": user, "UserNodes": nodes, "UserText": userText}, nil
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
		Username, Password string
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}
	if len(params.Username) < 3 {
		return httputils.JsonError{Code: 400, Error: "USERNAME_TO_SHORT"}, nil
	}
	_, err := core.RegisterUser(db, wr, params.Username, params.Password)
	if merry.Is(err, core.ErrUsernameExsists) {
		return httputils.JsonError{Code: 400, Error: "USERNAME_EXISTS"}, nil
	}
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return "ok", nil
}

func HandleAPILogin(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	params := &struct {
		Username, Password string
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}
	_, err := core.LoginUser(db, wr, params.Username, params.Password)
	if merry.Is(err, core.ErrUserNotFound) {
		return httputils.JsonError{Code: 403, Error: "WRONG_USERNAME_OR_PASSWORD"}, nil
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

func extractStartEndDatesStrFromQuery(query url.Values) (string, string) {
	startDateStr := query.Get("start_date")
	endDateStr := query.Get("end_date")
	nowStr := time.Now().In(time.UTC).Format("2006-01-02")
	endTime, err := time.ParseInLocation("2006-01-02", endDateStr, time.UTC)
	if err != nil {
		return nowStr, nowStr
	}
	startTime, err := time.ParseInLocation("2006-01-02", startDateStr, time.UTC)
	if err != nil {
		return nowStr, nowStr
	}
	if endTime.Sub(startTime) > 40*24*time.Hour {
		return nowStr, nowStr
	}
	return startDateStr, endDateStr
}

func HandleAPIUserNodePings(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	nodeID, err := storj.NodeIDFromString(ps.ByName("node_id"))
	if err != nil {
		return httputils.JsonError{Code: 400, Error: "NODE_ID_DECODE_ERROR", Description: err.Error()}, nil
	}

	startDateStr, endDateStr := extractStartEndDatesStrFromQuery(r.URL.Query())

	wr.Header().Set("Content-Type", "application/octet-stream")

	var histories []*core.UserNodeHistory
	histsQuery := db.Model(&histories).Column("pings", "date").
		Where("node_id = ? AND date BETWEEN ? AND ?", nodeID, startDateStr, endDateStr).
		Order("date")

	if strings.Contains(r.URL.Path, "/sat/") {
		histsQuery = histsQuery.Where("user_id = (SELECT id FROM users WHERE username = 'satellites')")
	} else {
		user := r.Context().Value(CtxKeyUser).(*core.User)
		histsQuery = histsQuery.Where("user_id = ?", user.ID)
	}

	if err = histsQuery.Select(); err != nil {
		return nil, merry.Wrap(err)
	}
	buf := make([]byte, 4+1440*2)
	for _, hist := range histories {
		binary.LittleEndian.PutUint32(buf, uint32(hist.Date.Unix()))
		for i, ping := range hist.Pings {
			buf[4+i*2+0] = byte(ping & 0xFF)
			buf[4+i*2+1] = byte(ping >> 8)
		}
		_, err := wr.Write(buf)
		if err != nil {
			return nil, merry.Wrap(err)
		}
	}
	return nil, nil
}

func HandleAPIUserTexts(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	user := r.Context().Value(CtxKeyUser).(*core.User)

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	_, err = db.Exec(`
		INSERT INTO user_texts (user_id, date, text, updated_at) VALUES (?, (NOW() at time zone 'utc')::date, ?, NOW())
		ON CONFLICT (user_id, date) DO UPDATE SET text = EXCLUDED.text, updated_at = NOW()`,
		user.ID, string(buf))
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return "ok", nil
}

func Handle404(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	tmplHnd := r.Context().Value(httputils.CtxKeyMain).(*httputils.MainCtx).TemplateHandler
	return merry.Wrap(tmplHnd.RenderTemplate(wr, r, map[string]interface{}{"FPath": "404.html"}))
}

func HandleHtml500(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	tmplHnd := r.Context().Value(httputils.CtxKeyMain).(*httputils.MainCtx).TemplateHandler
	return merry.Wrap(tmplHnd.RenderTemplate(wr, r, map[string]interface{}{"FPath": "500.html", "Block": "500.html"}))
}
