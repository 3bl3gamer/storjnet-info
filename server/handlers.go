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

func extractEndDateFromQuery(query url.Values) time.Time {
	endDateStr := query.Get("end_date")
	endTime, err := parseIntervalDate(endDateStr, true)
	if err != nil {
		_, endTime = defaultStartEndInterval()
	}
	return endTime
}

func extractEndDateStrFromQuery(query url.Values) string {
	return extractEndDateFromQuery(query).Format("2006-01-02")
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
	sats := r.Context().Value(CtxKeySatellites).(utils.Satellites)

	type SatInfo struct {
		Num   int64  `json:"num"`
		Label string `json:"label"`
		Quic  bool   `json:"quic"`
	}
	var satsInfo []SatInfo
	for i, sat := range sats {
		satsInfo = append(satsInfo, SatInfo{Num: int64(i), Label: sat.Label, Quic: sat.QUICDialer != nil})
	}

	return map[string]interface{}{"FPath": "ping_my_node.html", "UsableSats": satsInfo}, nil
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
	sats := r.Context().Value(CtxKeySatellites).(utils.Satellites)
	params := &struct {
		ID, Address  string
		DialOnly     bool
		Mode         string
		SatelliteNum int64
	}{}
	if jsonErr := unmarshalFromBody(r, params); jsonErr != nil {
		return *jsonErr, nil
	}

	id, err := storj.NodeIDFromString(params.ID)
	if err != nil {
		return httputils.JsonError{Code: 400, Error: "NODE_ID_DECODE_ERROR", Description: err.Error()}, nil
	}

	satMode := utils.SatModeTCP
	if params.Mode == "quic" {
		satMode = utils.SatModeQUIC
	}

	sat := sats[0]
	if params.SatelliteNum >= 0 && params.SatelliteNum < int64(len(sats)) {
		sat = sats[params.SatelliteNum]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stt := time.Now()
	conn, err := sat.Dial(ctx, params.Address, id, satMode)
	if err != nil {
		return httputils.JsonError{Code: 400, Error: "NODE_DIAL_ERROR", Description: err.Error()}, nil
	}
	dialDuration := time.Now().Sub(stt).Seconds()
	defer conn.Close()

	var pingDuration float64
	if !params.DialOnly {
		stt := time.Now()
		if err := sat.Ping(ctx, conn, satMode); err != nil && !utils.IsUntrustedSatPingError(err) {
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
	// sorting improves compression ratio: 55kb -> 24kb (85kb uncompressed original)
	// sorting by (jsonb)::float8 is faster than just by jsonb
	_, err := db.Query(&nodeLocations, `
		SELECT (location->'longitude')::float8 AS lon, (location->'latitude')::float8 AS lat
		FROM nodes WHERE location IS NOT NULL AND updated_at > NOW() - INTERVAL '1 day'
		ORDER BY (location->'latitude')::float8`)
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

	endDate := extractEndDateFromQuery(r.URL.Query())

	type TopItem struct {
		Country string `json:"country"`
		Count   int64  `json:"count"`
	}
	var stats struct {
		CountriesCount int64     `json:"countriesCount"`
		CountriesTop   []TopItem `json:"countriesTop"`
	}
	// do not QueryOne: there may be no data and empty (unchanged) stats should be returned
	_, err := db.Query(&stats, `
		SELECT
			(
				SELECT count(*) FILTER (WHERE key != '<unknown>')
				FROM jsonb_object_keys(countries) AS key
			) AS countries_count,
			(
				SELECT jsonb_agg(jsonb_build_object('country', (t).key, 'count', (t).value))
				FROM (
					SELECT t FROM jsonb_each(countries) AS t
					ORDER BY (t).value::int DESC
				) AS t
			) AS countries_top
		FROM node_stats
		WHERE created_at <= ?
		ORDER BY id DESC LIMIT 1
		`, endDate.AddDate(0, 0, 1))
	if err != nil {
		return nil, merry.Wrap(err)
	}
	if stats.CountriesTop == nil {
		stats.CountriesTop = []TopItem{}
	}
	return stats, nil
}

func HandleAPINodesSubnetSummary(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)

	endDate := extractEndDateFromQuery(r.URL.Query())

	type TopItem struct {
		Subnet string `json:"subnet"`
		Size   int64  `json:"size"`
	}
	type SizeItem struct {
		Size  int64 `json:"size"`
		Count int64 `json:"count"`
	}
	type IPTypeItem struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var stats struct {
		SubnetsCount int64        `json:"subnetsCount"`
		SubnetsTop   []TopItem    `json:"subnetsTop"`
		SubnetSizes  []SizeItem   `json:"subnetSizes"`
		IPTypes      []IPTypeItem `json:"ipTypes"`
	}
	// do not QueryOne: there may be no data and empty (unchanged) stats should be returned
	_, err := db.Query(&stats, `
		SELECT
			(
				SELECT jsonb_agg(jsonb_build_object('subnet', (t).key, 'size', (t).value))
				FROM (
					SELECT t FROM jsonb_each(subnets_top) AS t
					ORDER BY (t).value::int DESC
					LIMIT 5
				) AS t
			) AS subnets_top,
			(
				SELECT jsonb_agg(jsonb_build_object('size', (t).key::int, 'count', (t).value))
				FROM (
					SELECT t FROM jsonb_each(subnet_sizes) AS t
					ORDER BY (t).key::int ASC
				) AS t
			) AS subnet_sizes,
			(
				SELECT jsonb_agg(jsonb_build_object('type', (t).key, 'count', (t).value))
				FROM (
					SELECT t FROM jsonb_each(ip_types) AS t
					ORDER BY (t).value::int DESC
				) AS t
			) AS ip_types
		FROM node_stats
		WHERE created_at <= ?
		ORDER BY id DESC LIMIT 1
		`, endDate.AddDate(0, 0, 1))
	if err != nil {
		return nil, merry.Wrap(err)
	}
	for _, subnetSize := range stats.SubnetSizes {
		stats.SubnetsCount += subnetSize.Count
	}
	if stats.SubnetsTop == nil {
		stats.SubnetsTop = []TopItem{}
	}
	if stats.SubnetSizes == nil {
		stats.SubnetSizes = []SizeItem{}
	}
	if stats.IPTypes == nil {
		stats.IPTypes = []IPTypeItem{}
	}
	return stats, nil
}

type CountriesStatItem struct {
	name  string
	count int64
}
type CountriesStatItemList []CountriesStatItem
type CountriesStat struct {
	items CountriesStatItemList
}

func (list CountriesStatItemList) partition(left, right int) int {
	x := list[right].count
	i := left
	for j := left; j < right; j++ {
		if list[j].count >= x {
			if i != j {
				list[i], list[j] = list[j], list[i]
			}
			i += 1
		}
	}
	list[i], list[right] = list[right], list[i]
	return i
}
func (list CountriesStatItemList) moveLeftNLargest(left, right, n int) {
	if n > right-left+1 {
		return
	}
	partIndex := list.partition(left, right)

	if partIndex-left == n-1 {
		return
	}

	if partIndex-left > n-1 {
		// recur left subarray
		list.moveLeftNLargest(left, partIndex-1, n)
	} else {
		// recur right subarray
		list.moveLeftNLargest(partIndex+1, right, n-(partIndex+1-left))
	}
}
func (list CountriesStatItemList) MoveLeftNLargest(n int) {
	list.moveLeftNLargest(0, len(list)-1, n)
}

func (list CountriesStatItemList) findCountLeft(name string, fromIndex int) (int64, int) {
	for i := fromIndex; i >= 0; i-- {
		item := list[i]
		if item.name == name {
			return item.count, i
		}
	}
	return 0, -1
}
func (list CountriesStatItemList) findCountRight(name string, fromIndex int) (int64, int) {
	for i, item := range list[fromIndex:] {
		if item.name == name {
			return item.count, i + fromIndex
		}
	}
	return 0, -1
}

// Assumes Postgres JSONB keys are sorted by (len(key), key):
//
//	{"Chile": 9, "China": 86, "India": 100, "Italy": 107, ... "Bosnia and Herzegovina": 1, "Saint Pierre and Miquelon": 1}
//
// so each time country index is same or near prev index.
func (list CountriesStatItemList) FindCountFor(name string, prevIndex int) (int64, int) {
	if prevIndex != -1 && prevIndex < len(list) {
		indexName := list[prevIndex].name
		if indexName == name {
			return list[prevIndex].count, prevIndex
		}
		if len(name) < len(indexName) || (len(name) == len(indexName) && name < indexName) {
			return list.findCountLeft(name, prevIndex-1)
		}
		return list.findCountRight(name, prevIndex+1)
	}
	return list.findCountRight(name, 0)
}

// Expects {"Chile": 9, "China": 86, ...}
func (cs *CountriesStat) Scan(src interface{}) error {
	buf, ok := src.([]byte)
	if !ok {
		return merry.New("unexpected type")
	}
	if len(buf) == 0 || buf[0] != '{' {
		return merry.New("unexpected value start")
	}
	pos := 1
	stringStart := 0
	curString := ""
	readingValue := false
	curValue := int64(0)
	for ; pos < len(buf); pos++ {
		c := buf[pos]
		if c == '"' {
			if stringStart == 0 {
				stringStart = pos + 1
			} else {
				curString = string(buf[stringStart:pos])
				stringStart = 0
			}
			continue
		}
		if c == ':' && stringStart == 0 {
			stringStart = 0 //just in case
			readingValue = true
			curValue = 0
			continue
		}
		if readingValue && c >= '0' && c <= '9' {
			curValue = curValue*10 + int64(c-'0')
			continue
		}
		if (c == ',' || c == '}') && stringStart == 0 {
			if cs.items == nil {
				cs.items = make(CountriesStatItemList, 0, 128) //usually there are 90-100 countries
			}
			cs.items = append(cs.items, CountriesStatItem{name: curString, count: curValue})
			readingValue = false
		}
	}
	return nil
}

func HandleAPINodesCountries(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
	db := r.Context().Value(CtxKeyDB).(*pg.DB)

	query := r.URL.Query()
	startDate, endDate := extractStartEndDatesFromQuery(query, false)
	needAllCountries := query.Get("all") == "1"

	var items []struct {
		Stamp     int64
		Countries *CountriesStat
	}
	_, err := db.Query(&items, `
		SELECT
			countries,
			extract(epoch from created_at)::bigint AS stamp
		FROM node_stats WHERE created_at >= ? AND created_at < ?
		ORDER BY created_at
		`, startDate, endDate.AddDate(0, 0, 1))
	if err != nil {
		return nil, merry.Wrap(err)
	}

	var filterNames []string
	if needAllCountries {
		if len(items) > 0 {
			countries := items[0].Countries.items
			filterNames = make([]string, len(countries))
			for i, country := range countries {
				filterNames[i] = country.name
			}
		}
	} else {
		prevStamp := int64(0)
		countryFilterN := 15
		countryFilter := make(map[string]struct{}, countryFilterN*2)
		for i := len(items) - 1; i >= 0; i-- {
			item := items[i]
			if item.Stamp-prevStamp < 3*3600 {
				continue
			}
			countries := make(CountriesStatItemList, len(item.Countries.items))
			copy(countries, item.Countries.items)
			countries.MoveLeftNLargest(countryFilterN)
			for _, c := range countries[:countryFilterN] {
				countryFilter[c.name] = struct{}{}
			}
		}
		filterNames = make([]string, 0, len(countryFilter))
		for name := range countryFilter {
			filterNames = append(filterNames, name)
		}
	}

	maxNameLen := 0
	for _, name := range filterNames {
		if len(name) > maxNameLen {
			maxNameLen = len(name)
		}
	}

	startStamp := startDate.Unix()
	maxStamp := startStamp
	for _, item := range items {
		if item.Stamp > maxStamp {
			maxStamp = item.Stamp
		}
	}
	countsArrLen := int((maxStamp-startStamp)/3600 + 1)

	wr.Header().Set("Content-Type", "application/octet-stream")

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf, uint32(startStamp))
	binary.LittleEndian.PutUint32(buf[4:], uint32(countsArrLen))
	if _, err := wr.Write(buf); err != nil {
		return nil, merry.Wrap(err)
	}

	buf = make([]byte, 1+(maxNameLen+1)+2*countsArrLen)
	for _, name := range filterNames {
		// zeroing
		for i := range buf {
			buf[i] = 0
		}

		// country name
		buf[0] = byte(len(name))
		copy(buf[1:], []byte(name))

		valOffset := 1 + len(name)
		if valOffset%2 == 1 {
			valOffset += 1
		}
		// country counts
		prevIndex := -1
		for _, item := range items {
			i := int((item.Stamp - startStamp) / 3600)
			var count int64
			count, prevIndex = item.Countries.items.FindCountFor(name, prevIndex)
			buf[valOffset+2*i+0] = byte(count)
			buf[valOffset+2*i+1] = byte(count >> 8)
		}
		if _, err := wr.Write(buf[:valOffset+2*countsArrLen]); err != nil {
			return nil, merry.Wrap(err)
		}
	}
	return nil, nil
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

	var offCounts []struct{ Active, Stamp int64 }
	_, err = db.Query(&offCounts, `
		SELECT active_nodes AS active, extract(epoch from created_at)::bigint AS stamp
		FROM off_node_stats WHERE created_at >= ? AND created_at < ?`,
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
	for _, count := range offCounts {
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

	const COUNTS_ITEM_SIZE = 8
	const CHANGES_ITEM_SIZE = 4
	buf := make([]byte, 4+4+int(countsArrLen)*COUNTS_ITEM_SIZE+4+int(changesArrLen)*CHANGES_ITEM_SIZE)
	fullBuf := buf
	binary.LittleEndian.PutUint32(buf, uint32(startStamp))
	binary.LittleEndian.PutUint32(buf[4:], uint32(countsArrLen))
	buf = buf[4+4:]
	for _, count := range counts {
		i := (count.Stamp - startStamp) / 3600
		binary.LittleEndian.PutUint16(buf[i*COUNTS_ITEM_SIZE+0:], uint16(count.H05))
		binary.LittleEndian.PutUint16(buf[i*COUNTS_ITEM_SIZE+2:], uint16(count.H8))
		binary.LittleEndian.PutUint16(buf[i*COUNTS_ITEM_SIZE+4:], uint16(count.H24))
	}
	for _, count := range offCounts {
		i := (count.Stamp - startStamp) / 3600
		s := buf[i*COUNTS_ITEM_SIZE+6:]
		// finding max hour count among all satellites
		if uint16(count.Active) > binary.LittleEndian.Uint16(s) {
			binary.LittleEndian.PutUint16(s, uint16(count.Active))
		}
	}
	buf = buf[countsArrLen*COUNTS_ITEM_SIZE:]
	binary.LittleEndian.PutUint32(buf, uint32(changesArrLen))
	buf = buf[4:]
	for _, change := range changes {
		i := change.Delta
		binary.LittleEndian.PutUint16(buf[i*CHANGES_ITEM_SIZE+0:], uint16(change.Come))
		binary.LittleEndian.PutUint16(buf[i*CHANGES_ITEM_SIZE+2:], uint16(change.Left))
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
