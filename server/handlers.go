package server

import (
	"context"
	"database/sql"
	"encoding/binary"
	"io"
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
	"storj.io/common/storj"
)

func defaultStartEndInterval() (time.Time, time.Time) {
	now := time.Now().In(time.UTC)
	year, month, _ := now.Date()
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, -1)
	return monthStart, monthEnd
}

func parseIntervalDate(str string, isEnd bool) (time.Time, error) {
	res, err := time.ParseInLocation("2006-1-2", str, time.UTC)
	if err != nil {
		res, err = time.ParseInLocation("2006-1", str, time.UTC)
		if isEnd && err != nil {
			res = res.AddDate(0, 1, 1-res.Day())
		}
	}
	if err != nil {
		return time.Time{}, merry.Wrap(err)
	}
	return res, nil
}

func extractStartEndDatesFromQuery(query url.Values, shortKeys bool) (time.Time, time.Time) {
	var startDateStr, endDateStr string
	if shortKeys {
		startDateStr = query.Get("start")
		endDateStr = query.Get("end")
	} else {
		startDateStr = query.Get("start_date")
		endDateStr = query.Get("end_date")
	}
	endTime, err := parseIntervalDate(endDateStr, true)
	if err != nil {
		return defaultStartEndInterval()
	}
	startTime, err := parseIntervalDate(startDateStr, false)
	if err != nil {
		return defaultStartEndInterval()
	}
	delta := endTime.Sub(startTime)
	if delta > 94*24*time.Hour || delta < 20*time.Hour {
		return defaultStartEndInterval()
	}
	return startTime, endTime
}

func extractStartEndDatesStrFromQuery(query url.Values, shortKeys bool) (string, string) {
	startTime, endTime := extractStartEndDatesFromQuery(query, shortKeys)
	return startTime.Format("2006-01-02"), endTime.Format("2006-01-02")
}

func HandleIndex(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	user := r.Context().Value(CtxKeyUser).(*core.User)
	startDate, endDate := defaultStartEndInterval()
	nodes, err := core.LoadSatNodes(db, startDate, endDate)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return map[string]interface{}{"FPath": "index.html", "User": user, "SatNodes": nodes}, nil
}

func HandlePingMyNode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	return map[string]interface{}{"FPath": "ping_my_node.html"}, nil
}

func HandleNeighbors(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	return map[string]interface{}{"FPath": "neighbors.html"}, nil
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
	return map[string]interface{}{
		"FPath":      "user_dashboard.html",
		"User":       user,
		"UserNodes":  nodes,
		"UserText":   userText,
		"ServerTime": time.Now(),
	}, nil
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

func HandleAPINeighbors(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	subnet := ps.ByName("subnet")

	var count int64
	_, err := db.QueryOne(&count, `
		SELECT count(*) FROM nodes
		WHERE node_ip_subnet(ip_addr) = node_ip_subnet(?::inet)
		  AND updated_at > NOW() - INTERVAL '1 day'`, subnet)
	if err != nil {
		if perr, ok := merry.Unwrap(err).(pg.Error); ok {
			if strings.HasPrefix(perr.Field('M'), "invalid input syntax for type inet") {
				return httputils.JsonError{Code: 400, Error: "WRONG_SUBNET_FORMAT"}, nil
			}
		}
		return nil, merry.Wrap(err)
	}
	return map[string]interface{}{"count": count}, nil
}

func HandleAPINeighborsExt(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	params := &struct {
		Subnets   []string
		MyNodeIDs []storj.NodeID
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}

	items := []*struct {
		Subnet            string `json:"subnet"`
		NodesTotal        int64  `json:"nodesTotal"`
		ForeignNodesCount int64  `json:"foreignNodesCount"`
	}{}
	_, err := db.Query(&items, `
		SELECT host(node_ip_subnet(ip_addr)) AS subnet,
			count(*) AS nodes_total,
			count(*) FILTER (WHERE NOT (id = ANY(?))) AS foreign_nodes_count
		FROM nodes
		WHERE node_ip_subnet(ip_addr) IN (SELECT node_ip_subnet(t) FROM unnest(ARRAY[?]::inet[]) AS t)
		  AND updated_at > NOW() - INTERVAL '1 day'
		GROUP BY node_ip_subnet(ip_addr)`, pg.Array(params.MyNodeIDs), pg.In(params.Subnets))
	if err != nil {
		if perr, ok := merry.Unwrap(err).(pg.Error); ok {
			if strings.HasPrefix(perr.Field('M'), "invalid input syntax for type inet") {
				return httputils.JsonError{Code: 400, Error: "WRONG_SUBNET_FORMAT"}, nil
			}
		}
		return nil, merry.Wrap(err)
	}
	return map[string]interface{}{"counts": items}, nil
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

func HandleAPIGetSatNodes(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	startDate, endDate := extractStartEndDatesFromQuery(r.URL.Query(), false)
	return core.LoadSatNodes(db, startDate, endDate)
}

func HandleAPIUserNodePings(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	nodeID, err := storj.NodeIDFromString(ps.ByName("node_id"))
	if err != nil {
		return httputils.JsonError{Code: 400, Error: "NODE_ID_DECODE_ERROR", Description: err.Error()}, nil
	}

	query := r.URL.Query()
	startDateStr, endDateStr := extractStartEndDatesStrFromQuery(query, false)
	fullPingsData := query.Get("full") == "1"

	var histories []*core.UserNodeHistory
	histsQuery := db.Model(&histories).Column("pings", "date").
		Where("node_id = ? AND date BETWEEN ? AND ?", nodeID, startDateStr, endDateStr).
		Order("date")

	if strings.Contains(r.URL.Path, "/sat/") {
		histsQuery = histsQuery.
			Where("user_id = (SELECT id FROM users WHERE username = 'satellites')")
	} else {
		user := r.Context().Value(CtxKeyUser).(*core.User)
		histsQuery = histsQuery.Where("user_id = ?", user.ID)
	}

	if err = histsQuery.Select(); err != nil {
		return nil, merry.Wrap(err)
	}

	wr.Header().Set("Content-Type", "application/octet-stream")
	if fullPingsData {
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
	} else {
		buf := make([]byte, 4+1440)
		for _, hist := range histories {
			binary.LittleEndian.PutUint32(buf, uint32(hist.Date.Unix()))
			for i, ping := range hist.Pings {
				val := int(ping) % 2000
				if val > 1 {
					val = val * 256 / 2000
					if val <= 1 {
						val = 2
					}
				}
				buf[4+i] = byte(val)
			}
			_, err := wr.Write(buf)
			if err != nil {
				return nil, merry.Wrap(err)
			}
		}
	}
	return nil, nil
}

func HandleAPIUserTexts(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)
	user := r.Context().Value(CtxKeyUser).(*core.User)

	buf, err := io.ReadAll(r.Body)
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

func HandleAPIStorjTokenTxSummary(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)

	startDateStr, endDateStr := extractStartEndDatesStrFromQuery(r.URL.Query(), false)

	var daySums []*core.StorjTokenTxSummary
	err := db.Model(&daySums).
		Where("date BETWEEN ? AND ?", startDateStr, endDateStr).
		Order("date").Select()
	if err != nil {
		return nil, merry.Wrap(err)
	}

	wr.Header().Set("Content-Type", "application/octet-stream")
	buf := make([]byte, 4+24*(4+4+4+4))
	for _, day := range daySums {
		binary.LittleEndian.PutUint32(buf, uint32(day.Date.Unix()))
		utils.CopyFloat32SliceToBuf(buf[4+24*4*0:], day.Preparings)
		utils.CopyFloat32SliceToBuf(buf[4+24*4*1:], day.Payouts)
		utils.CopyInt32SliceToBuf(buf[4+24*4*2:], day.PayoutCounts)
		utils.CopyFloat32SliceToBuf(buf[4+24*4*3:], day.Withdrawals)
		_, err := wr.Write(buf)
		if err != nil {
			return nil, merry.Wrap(err)
		}
	}
	return nil, nil
}

func HandleAPINodesLocations(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(ctxKey("db")).(*pg.DB)

	var nodeLocations []struct{ Lon, Lat float32 }
	_, err := db.Query(&nodeLocations, `
		SELECT (location->'longitude')::float8 AS lon, (location->'latitude')::float8 AS lat
		FROM nodes WHERE location IS NOT NULL AND updated_at > NOW() - INTERVAL '1 day'`)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	buf := make([]byte, len(nodeLocations)*4)
	for i, loc := range nodeLocations {
		lon := uint16((180 + loc.Lon) / 360 * 65536)
		lat := uint16((90 + loc.Lat) / 180 * 65536)
		buf[i*4+0] = byte(lon % 256)
		buf[i*4+1] = byte(lon >> 8)
		buf[i*4+2] = byte(lat % 256)
		buf[i*4+3] = byte(lat >> 8)
	}

	wr.Header().Set("Content-Type", "application/octet-stream")
	_, err = wr.Write(buf)
	return nil, merry.Wrap(err)
}

func HandleAPINodesLocationSummary(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)

	var stats struct {
		CountriesCount int64 `json:"countriesCount"`
		CountriesTop   []struct {
			Country string `json:"country"`
			Count   int64  `json:"count"`
		} `json:"countriesTop"`
	}
	_, err := db.Query(&stats, `
		SELECT
			(SELECT count(*) FROM jsonb_object_keys(countries)) AS countries_count,
			(
				SELECT jsonb_agg(jsonb_build_object('country', (t).key, 'count', (t).value))
				FROM (
					SELECT t FROM jsonb_each(countries) AS t
					ORDER BY (t).value::int DESC limit 10
				) AS t
			) AS countries_top
		FROM node_stats ORDER BY id DESC LIMIT 1
		`)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return stats, nil
}

func HandleAPINodesCounts(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)

	startDate, endDate := extractStartEndDatesFromQuery(r.URL.Query(), false)

	var counts []struct{ H05, H8, H24, Stamp int64 }
	_, err := db.Query(&counts, `
		SELECT
			(active_count_hours->'0.5')::int AS h05,
			(active_count_hours->'8')::int AS h8,
			(active_count_hours->'24')::int AS h24,
			extract(epoch from created_at)::bigint AS stamp
		FROM node_stats WHERE created_at >= ? AND created_at < ?`,
		startDate, endDate.AddDate(0, 0, 1))
	if err != nil {
		return nil, merry.Wrap(err)
	}

	var changes []struct {
		Date       time.Time
		Delta      int64
		Left, Come int64
	}
	_, err = db.Query(&changes, `
		SELECT date,
			COALESCE(array_length(left_node_ids, 1), 0) AS left,
			COALESCE(array_length(come_node_ids, 1), 0) AS come
		FROM node_daily_stats
		WHERE kind = 'active'
		  AND date BETWEEN ?::date AND ?::date`,
		startDate, endDate)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	startStamp := startDate.Unix()
	maxStamp := startStamp
	for _, count := range counts {
		if count.Stamp > maxStamp {
			maxStamp = count.Stamp
		}
	}
	countsArrLen := (maxStamp-startStamp)/3600 + 1

	changesArrLen := int64(0)
	for i, change := range changes {
		// dates come as UTC's, checking just in case
		if change.Date.Location() != time.UTC || change.Date.Hour() != 0 {
			panic("date is not UTC midnight")
		}
		delta := int64(change.Date.In(time.UTC).Sub(startDate).Hours() / 24)
		if delta+1 > changesArrLen {
			changesArrLen = delta + 1
		}
		changes[i].Delta = delta
	}

	buf := make([]byte, 4+4+int(countsArrLen)*6+4+int(changesArrLen)*4)
	fullBuf := buf
	binary.LittleEndian.PutUint32(buf, uint32(startStamp))
	binary.LittleEndian.PutUint32(buf[4:], uint32(countsArrLen))
	buf = buf[8:]
	for _, count := range counts {
		i := (count.Stamp - startStamp) / 3600
		buf[i*6+0] = byte(count.H05)
		buf[i*6+1] = byte(count.H05 >> 8)
		buf[i*6+2] = byte(count.H8)
		buf[i*6+3] = byte(count.H8 >> 8)
		buf[i*6+4] = byte(count.H24)
		buf[i*6+5] = byte(count.H24 >> 8)
	}
	buf = buf[countsArrLen*6:]
	binary.LittleEndian.PutUint32(buf, uint32(changesArrLen))
	buf = buf[4:]
	for _, change := range changes {
		i := change.Delta
		buf[i*4+0] = byte(change.Come)
		buf[i*4+1] = byte(change.Come >> 8)
		buf[i*4+2] = byte(change.Left)
		buf[i*4+3] = byte(change.Left >> 8)
	}

	wr.Header().Set("Content-Type", "application/octet-stream")
	_, err = wr.Write(fullBuf)
	return nil, merry.Wrap(err)
}

func HandleAPIClientErrors(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)

	user := r.Context().Value(CtxKeyUser).(*core.User)
	var userID sql.NullInt64
	if user != nil {
		userID = sql.NullInt64{Int64: user.ID, Valid: true}
	}

	params := &struct {
		URL     string
		Message string
		Stack   string
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}

	_, err := db.Exec(
		"INSERT INTO client_errors (url, user_id, user_agent, lang, message, stack) VALUES (?,?,?,?,?,?)",
		params.URL, userID, r.UserAgent(), langFromRequest(r), params.Message, params.Stack)
	return nil, merry.Wrap(err)
}

func Handle404(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	tmplHnd := r.Context().Value(httputils.CtxKeyMain).(*httputils.MainCtx).TemplateHandler
	return merry.Wrap(tmplHnd.RenderTemplate(wr, r, map[string]interface{}{"FPath": "404.html"}))
}

func HandleHtml500(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	tmplHnd := r.Context().Value(httputils.CtxKeyMain).(*httputils.MainCtx).TemplateHandler
	return merry.Wrap(tmplHnd.RenderTemplate(wr, r, map[string]interface{}{"FPath": "500.html", "Block": "500.html"}))
}
