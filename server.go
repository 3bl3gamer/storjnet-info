package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/julienschmidt/httprouter"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type ctxKey string

var templateCache = make(map[string]*template.Template)

func pluralize(val int64, lang string, words ...string) string {
	if val < 0 {
		val = -val
	}
	d0 := val % 10
	d10 := val % 100
	switch lang {
	case "ru":
		if d10 == 11 || d10 == 12 || d0 == 0 || (d0 >= 5 && d0 <= 9) {
			return words[2]
		}
		if d0 >= 2 && d0 <= 4 {
			return words[1]
		}
		return words[0]
	default:
		if d10 == 11 || d10 == 12 || d0 == 0 || (d0 >= 2 && d0 <= 9) {
			return words[1]
		}
		return words[0]
	}
}

var sizeIBNames = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}

var monthNames = map[string][]string{
	"en": []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"},
	"ru": []string{"январь", "февраль", "март", "апрель", "май", "июнь", "июль", "август", "сентябрь", "октябрь", "ноябрь", "декабрь"},
}

func sizeIB(size int64) string {
	i := 0
	for ; i < len(sizeIBNames)-1 && 1<<uint(i*10+10) < size; i++ {
	}
	value := float64(size) / float64(int(1)<<uint(i*10))
	prec := 3
	if value >= 1000 {
		prec = 4 // чтоб 1000-1023 не отображалось через экспоненту
	}
	return strconv.FormatFloat(value, 'g', prec, 64) + " " + sizeIBNames[i]
}

type L10nUtls struct {
	Lang string
}

type L10nUtlsWithVal struct {
	L L10nUtls
	V interface{}
}

func (l L10nUtls) Is(lang string) bool {
	return l.Lang == lang
}

func (l L10nUtls) With(val interface{}) L10nUtlsWithVal {
	return L10nUtlsWithVal{L: l, V: val}
}

func (l L10nUtls) Loc(en string, special ...string) string {
	if l.Lang == "en" {
		return en
	}
	for i := 0; i < len(special)-1; i += 2 {
		if l.Lang == special[i] {
			return special[i+1]
		}
	}
	return en
}

func (l L10nUtls) FormatDateTime(t time.Time) string {
	t = t.In(time.UTC)
	switch l.Lang {
	case "ru":
		return t.Format("02.01.2006 в 15:04 UTC")
	default:
		return t.Format("2006.01.02 at 15:04 UTC")
	}
}

func (l L10nUtls) DateTimeTag(t time.Time) template.HTML {
	dt := template.HTMLEscapeString(l.FormatDateTime(t))
	stamp := t.In(time.UTC).Format(time.RFC3339)
	return template.HTML(`<time datetime="` + stamp + `">` + dt + `</time>`)
}

func (l L10nUtls) DateTimeMonth(t time.Time) string {
	if names, ok := monthNames[l.Lang]; ok {
		return names[t.Month()-1]
	}
	return monthNames["en"][t.Month()-1]
}

func (l L10nUtls) Ago(t time.Time) string {
	delta := time.Now().Sub(t)
	days := int64(delta / (24 * time.Hour))
	hours := int64((delta / time.Hour) % 24)
	minutes := int64((delta / time.Minute) % 60)
	switch l.Lang {
	case "ru":
		res := fmt.Sprintf("%d мин", minutes)
		if hours != 0 {
			res = fmt.Sprintf("%d ч %s", hours, res)
		}
		if days != 0 {
			res = fmt.Sprintf("%d д %s", days, res)
		}
		return res
	default:
		res := fmt.Sprintf("%d min", minutes)
		if hours != 0 {
			res = fmt.Sprintf("%d h %s", hours, res)
		}
		if days != 0 {
			res = fmt.Sprintf("%d d %s", days, res)
		}
		return res
	}
}

func (l L10nUtls) Pluralize(valI interface{}, words ...string) string {
	var val int64
	switch v := valI.(type) {
	case int:
		val = int64(v)
	default:
		val = v.(int64)
	}
	return pluralize(val, l.Lang, words...)
}

var templateFuncs = template.FuncMap{
	"formatDateISO": func(t time.Time) string {
		return t.Format("2006-01-02")
	},
	"sub": func(a, b int64) int64 {
		return a - b
	},
	// "div": func(a, b int64) int64 {
	// 	return a / b
	// },
	"percent": func(a, b int64) float64 {
		return math.Round(float64(a)/float64(b)*1000) / 10
	},
	"signed": func(a int64) string {
		res := strconv.FormatInt(a, 10)
		if a >= 0 {
			res = "+" + res
		}
		return res
	},
	"sizeIB": sizeIB,
	"sizeIBSign": func(size int64) string {
		if size >= 0 {
			return "+" + sizeIB(size)
		}
		return "-" + sizeIB(-size)
	},
	"percents": func(val float64) string {
		return fmt.Sprintf("%g", val*100)
	},
	"nodeTypeName": func(nodeType int64) string {
		return pb.NodeType_name[int32(nodeType)]
	},
	"formattedJSON": func(val interface{}) (string, error) {
		buf, err := json.MarshalIndent(val, "", " ")
		if err != nil {
			return "", merry.Wrap(err)
		}
		return string(buf), nil
	},
	"marshaledJSON": func(val interface{}) (string, error) {
		buf, err := json.Marshal(val)
		if err != nil {
			return "", merry.Wrap(err)
		}
		return string(buf), nil
	},
	"nodeIDHex": func(id storj.NodeID) string {
		return hex.EncodeToString(id[:])
	},
	"nodeIDBin": func(id storj.NodeID) string {
		res := ""
		for _, b := range id {
			res += fmt.Sprintf("% 08b", b)
		}
		return res
	},
}

type QueryExt struct {
	url.Values
}

func (q QueryExt) EncodeWithParam(key, value string) template.URL {
	q.Values.Set(key, value)
	return template.URL(q.Values.Encode())
}

func extractMonthStartTimeUTC(query url.Values) time.Time {
	startTime, err := time.ParseInLocation("2006-01-02", query.Get("stats_from"), time.UTC)
	if err != nil {
		startTime = time.Now().UTC()
	}
	year, month, _ := startTime.Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
}

func getTemplate(path string) (*template.Template, error) {
	// TODO: concurrency
	if envMode == "prod" {
		if tmpl, ok := templateCache[path]; ok {
			return tmpl, nil
		}
	}

	logInfo("SERVER", "building template: %s", path)
	tmpl := template.New(path)
	tmpl = tmpl.Funcs(templateFuncs)
	tmpl, err := tmpl.ParseGlob("www/templates/_*.html")
	if err != nil {
		return nil, merry.Wrap(err)
	}
	tmpl, err = tmpl.ParseFiles("www/templates/" + path)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	templateCache[path] = tmpl
	return tmpl, nil
}

func langOrDefault(lang string) string {
	if lang == "en" || lang == "ru" {
		return lang
	}
	return "en"
}
func langFromRequest(r *http.Request) string {
	if c, err := r.Cookie("lang"); err == nil {
		return langOrDefault(c.Value)
	}
	if langs := r.Header.Get("Accept-Language"); len(langs) > 2 {
		// TODO: maybe support smth like "Accept-Language: fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5"
		return langOrDefault(langs[:2])
	}
	return "en"
}

func render(wr http.ResponseWriter, r *http.Request, statusCode int, tmplName, blockName string, data map[string]interface{}) error {
	if data == nil {
		data = map[string]interface{}{}
	}
	data["L"] = L10nUtls{Lang: langFromRequest(r)}
	tmpl, err := getTemplate(tmplName)
	if err != nil {
		return merry.Wrap(err)
	}
	wr.Header().Set("Content-Type", "text/html")
	wr.WriteHeader(statusCode)
	return merry.Wrap(tmpl.ExecuteTemplate(wr, blockName, data))
}

type HandleExt func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error

func wrap(db *pg.DB, handle HandleExt) httprouter.Handle {
	return func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if r.Method == "GET" {
			// всякая яндексметрика режется адблоками, так что считаем посещения и со своей стороны
			go func() {
				ipAddress := r.Header.Get("X-Real-IP")
				if ipAddress == "" {
					ipAddress = r.RemoteAddr[:strings.LastIndex(r.RemoteAddr, ":")]
				}
				if err := SaveVisit(db, ipAddress, r.UserAgent(), r.URL.Path); err != nil {
					logErr("SERVER", merry.Details(err))
				}
			}()
		}

		r = r.WithContext(context.WithValue(r.Context(), ctxKey("db"), db))
		if err := handle(wr, r, ps); err != nil {
			d := merry.Details(err)
			logErr("SERVER", d)
			if envMode == "prod" {
				Handle500(wr, r, ps)
			} else {
				wr.Write([]byte("<pre>" + d))
			}
		}
	}
}

func Handle404(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	return render(wr, r, http.StatusNotFound, "404.html", "base", nil)
}

func Handle500(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	return render(wr, r, http.StatusInternalServerError, "500.html", "500.html", nil)
}

func HandleExplode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	return merry.New("KABOOM!")
}

func HandleIndex(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	db := r.Context().Value(ctxKey("db")).(*pg.DB)

	lastStat := &GlobalStat{}
	dayAgoStat := &GlobalStat{}
	err := db.Model(lastStat).Order("id DESC").Limit(1).Select()
	if err != nil {
		return merry.Wrap(err)
	}
	err = db.Model(dayAgoStat).Where("created_at < ?::timestamptz - INTERVAL '23.5 hours'", lastStat.CreatedAt).Order("id DESC").Limit(1).Select()
	if err != nil {
		return merry.Wrap(err)
	}

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	globalHistoryData, err := LoadGlobalNodesHistoryData(db, days)
	if err != nil {
		return merry.Wrap(err)
	}

	orderedTypes := []pb.NodeType{pb.NodeType_SATELLITE, pb.NodeType_BOOTSTRAP} //pb.NodeType_UPLINK, pb.NodeType_INVALID, pb.NodeType_STORAGE
	var nodeIDsWithType []struct {
		NodeType pb.NodeType
		IDs      []NodeIDExt `sql:"ids,array"`
	}
	_, err = db.Query(&nodeIDsWithType, `
		SELECT node_type, ids FROM (
			SELECT (self_params->'type')::int AS node_type, array_agg(id ORDER BY id) AS ids
			FROM nodes
			WHERE self_updated_at > '2019-10-06T00:00Z'::timestamptz - INTERVAL '24 hours' --NOW()
			AND self_params->'type' IS NOT NULL AND (self_params->'type')::int != ?
			GROUP BY (self_params->'type')::int
		) AS tt
		JOIN unnest(?::int[]) WITH ORDINALITY AS t(node_type, ord) USING(node_type) ORDER BY t.ord
		`, pb.NodeType_STORAGE, pg.Array(orderedTypes))
	if err != nil {
		return merry.Wrap(err)
	}

	return render(wr, r, http.StatusOK, "index.html", "base", map[string]interface{}{
		"LastStat":          lastStat,
		"DayAgoStat":        dayAgoStat,
		"NodeIDsWithType":   nodeIDsWithType,
		"NodeType_STORAGE":  pb.NodeType_STORAGE,
		"GlobalHistoryData": globalHistoryData,
		"ShowChartsClose":   days == 125,
	})
}

func HandleNode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	db := r.Context().Value(ctxKey("db")).(*pg.DB)

	nodeIDStr := ps.ByName("nodeID")
	nodeID, err := storj.NodeIDFromString(nodeIDStr)
	if err != nil {
		return render(wr, r, http.StatusBadRequest, "node.html", "base", map[string]interface{}{
			"NodeIDStr":       nodeIDStr,
			"NodeIDIsInvalid": true,
			"NodeIDDecodeErr": err.Error(),
		})
	}

	node := &Node{ID: nodeID}
	err = db.Model(node).
		Column("created_at", "kad_params", "kad_updated_at", "self_updated_at", "self_params", "last_ip", "location").
		WherePK().Select()
	if err == pg.ErrNoRows {
		appendFileString("unknown_nodes_log.txt", time.Now().Format("2006-01-02 15:04:05")+" "+node.ID.String()+"\n")
		return render(wr, r, http.StatusBadRequest, "node.html", "base", map[string]interface{}{
			"NodeIDStr":      node.ID.String(),
			"NodeIDNotFound": true,
			"Node":           node,
		})
	}
	if err != nil {
		return merry.Wrap(err)
	}

	subnetNeighborsCount := -1
	if node.LastIP != nil {
		_, err = db.QueryOne(&subnetNeighborsCount, `
			SELECT count(*) FROM nodes
			WHERE node_last_ip_subnet(last_ip) = node_last_ip_subnet(?)
			  AND id != ?
			  AND self_updated_at > NOW() - INTERVAL '24 hours'
			`, node.LastIP, node.ID)
		if err != nil {
			return merry.Wrap(err)
		}
	}

	query := r.URL.Query()
	statsTimeFrom := extractMonthStartTimeUTC(query)
	var dailyHistories []*NodeHistory
	err = db.Model(&dailyHistories).
		Where("id = ? AND date BETWEEN ?::date AND (?::date + INTERVAL '1 month')", node.ID, statsTimeFrom, statsTimeFrom).
		Order("date").
		Select()
	if err != nil && err != pg.ErrNoRows {
		return merry.Wrap(err)
	}

	statsPrevMonthTime := statsTimeFrom.AddDate(0, -1, 0)
	statsNextMonthTime := statsTimeFrom.AddDate(0, 1, 0)

	return render(wr, r, http.StatusOK, "node.html", "base", map[string]interface{}{
		"NodeIDStr":            node.ID.String(),
		"Node":                 node,
		"NodeHistory":          GroupNodeHistories(dailyHistories),
		"NodeType_STORAGE":     pb.NodeType_STORAGE,
		"StatsTimeFrom":        statsTimeFrom,
		"StatsPrevMonthTime":   statsPrevMonthTime,
		"StatsNextMonthTime":   statsNextMonthTime,
		"SubnetNeighborsCount": subnetNeighborsCount,
		"Query":                QueryExt{query},
	})
}

func HandleSearch(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	q := r.URL.Query().Get("q")
	location := "/"
	if q != "" {
		location = "/@" + q
	}
	http.Redirect(wr, r, location, 303)
	return nil
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

func HandleNodeLocations(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	db := r.Context().Value(ctxKey("db")).(*pg.DB)

	var nodeLocations []struct{ Lon, Lat float32 }
	_, err := db.Query(&nodeLocations, "SELECT (location).longitude AS lon, (location).latitude AS lat FROM nodes WHERE location IS NOT NULL")
	if err != nil {
		return merry.Wrap(err)
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
	_, err = wr.Write(buf)
	return merry.Wrap(err)
}

func StartHTTPServer(address string) error {
	db := makePGConnection()

	router := httprouter.New()
	router.Handle("GET", "/", wrap(db, HandleIndex))
	router.Handle("GET", "/@:nodeID", wrap(db, HandleNode))
	router.Handle("GET", "/search", wrap(db, HandleSearch))
	router.Handle("POST", "/lang", wrap(db, HandleLang))
	router.Handle("GET", "/node_locations.bin", wrap(db, HandleNodeLocations))
	router.Handle("GET", "/explode", wrap(db, HandleExplode))

	router.ServeFiles("/js/*filepath", http.Dir("www/js"))
	router.ServeFiles("/css/*filepath", http.Dir("www/css"))

	wrapped404 := wrap(db, Handle404)
	router.NotFound = http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		wrapped404(wr, r, httprouter.Params{{}})
	})
	log.Print("starting HTTP server on " + address)
	return merry.Wrap(http.ListenAndServe(address, router))
}
