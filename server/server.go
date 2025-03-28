package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"storjnet/core"
	"storjnet/utils"
	"storjnet/utils/storjutils"
	"strings"
	"time"

	httputils "github.com/3bl3gamer/go-http-utils"
	"github.com/ansel1/merry"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
)

type ctxKey string

const CtxKeySatellites = ctxKey("satellites")
const CtxKeyEnv = ctxKey("env")
const CtxKeyDB = ctxKey("db")
const CtxKeyGeoIPDB = ctxKey("geoip-db")
const CtxKeyUser = ctxKey("user")

func unmarshalFromBody(r *http.Request, obj interface{}) *httputils.JsonError {
	if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
		descr := ""
		if r.Context().Value(CtxKeyEnv).(utils.Env).IsDev() {
			descr = err.Error()
		}
		return &httputils.JsonError{Code: 400, Error: "JSON_DECODE_ERROR", Description: descr}
	}
	return nil
}
func unmarshalNodeFromBody(r *http.Request, node *core.Node) *httputils.JsonError {
	if err := json.NewDecoder(r.Body).Decode(node); err != nil {
		if strings.HasPrefix(err.Error(), "node ID error") {
			return &httputils.JsonError{Code: 400, Error: "NODE_ID_DECODE_ERROR", Description: err.Error()}
		}
		descr := ""
		if r.Context().Value(CtxKeyEnv).(utils.Env).IsDev() {
			descr = err.Error()
		}
		return &httputils.JsonError{Code: 400, Error: "JSON_DECODE_ERROR", Description: descr}
	}
	return nil
}

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

var monthNames = map[string][]string{
	"en": {"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"},
	"ru": {"январь", "февраль", "март", "апрель", "май", "июнь", "июль", "август", "сентябрь", "октябрь", "ноябрь", "декабрь"},
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
	delta := time.Since(t)
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

func chooseLang(lang string, langWithCountry string) string {
	if lang == "uk" || //Ukrainian
		lang == "be" || //Belarusian
		lang == "et" || //Estonian
		lang == "lz" || //Latvian
		lang == "lt" || //Lithuanian
		langWithCountry == "ro-MD" || //Romanian (Moldova)
		lang == "tk" || //Turkmen
		lang == "kk" || //Kazakh
		lang == "uz" || //Uzbek
		lang == "ky" || //Kyrgyz
		lang == "tg" || //Tajik
		lang == "hy" || //Armenian
		lang == "ka" { //Georgian
		return "ru"
	}
	if lang == "en" || lang == "ru" {
		return lang
	}
	return "en"
}
func langFromRequest(r *http.Request) string {
	if c, err := r.Cookie("lang"); err == nil {
		return chooseLang(c.Value, "")
	}
	if langs := r.Header.Get("Accept-Language"); len(langs) >= 2 {
		// TODO: maybe support smth like "Accept-Language: fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5"
		withCountry := ""
		if len(langs) >= 5 {
			withCountry = langs[:5]
		}
		return chooseLang(langs[:2], withCountry)
	}
	return "en"
}

func StartHTTPServer(address string, env utils.Env) error {
	ex, err := os.Executable()
	if err != nil {
		return merry.Wrap(err)
	}
	baseDir := filepath.Dir(ex)

	var bundleFPath, stylesFPath string

	db := utils.MakePGConnection()

	gdb, err := utils.OpenGeoIPConn("GeoLite2-City.mmdb")
	if err != nil {
		return merry.Wrap(err)
	}

	sats, err := storjutils.SatellitesSetUpFromEnv()
	if err != nil {
		return merry.Wrap(err)
	}

	// Config
	wrapper := &httputils.Wrapper{
		ShowErrorDetails: env.IsDev(),
		ExtraChainItem: func(handle httputils.HandlerExt) httputils.HandlerExt {
			return func(wr http.ResponseWriter, r *http.Request, params httprouter.Params) error {
				log.Debug().Str("method", r.Method).Str("path", r.URL.Path).Msg("request")
				r = r.WithContext(context.WithValue(r.Context(), CtxKeySatellites, sats))
				r = r.WithContext(context.WithValue(r.Context(), CtxKeyEnv, env))
				r = r.WithContext(context.WithValue(r.Context(), CtxKeyDB, db))
				r = r.WithContext(context.WithValue(r.Context(), CtxKeyGeoIPDB, gdb))
				return merry.Wrap(handle(wr, r, params))
			}
		},
		TemplateHandler: &httputils.TemplateHandler{
			CacheParsed: env.IsProd(),
			BasePath:    baseDir + "/www/templates",
			FuncMap:     template.FuncMap{},
			ParamsFunc: func(r *http.Request, ctx *httputils.MainCtx, params httputils.TemplateCtx) error {
				params["L"] = L10nUtls{Lang: langFromRequest(r)}
				params["BundleFPath"] = bundleFPath
				params["StylesFPath"] = stylesFPath
				return nil
			},
			LogBuild: func(path string) { log.Info().Str("path", path).Msg("building template") },
		},
		HandleHtml500: HandleHtml500,
		LogError: func(err error, r *http.Request) {
			// if errors.Is(err, syscall.EPIPE) {
			// 	log.Warn().Str("method", r.Method).Str("path", r.URL.Path).Msg("broken pipe")
			// } else {
			log.Error().Stack().Err(err).Str("method", r.Method).Str("path", r.URL.Path).Msg("")
			// }
		},
	}

	router := httprouter.New()
	route := func(method, path string, chain ...interface{}) {
		router.Handle(method, path, wrapper.WrapChain(chain...))
	}

	// Routes
	route("GET", "/", WithOptUser, HandleIndex)
	route("GET", "/ping_my_node", HandlePingMyNode)
	route("GET", "/neighbors", HandleNeighbors)
	route("GET", "/sanctions", HandleSanctions)
	route("GET", "/~", WithOptUser, HandleUserDashboard)

	route("POST", "/lang", HandleLang)
	route("POST", "/api/register", HandleAPIRegister)
	route("POST", "/api/login", HandleAPILogin)
	route("POST", "/api/ping_my_node", HandleAPIPingMyNode)
	route("GET", "/api/neighbors/:subnet", HandleAPINeighbors)
	route("POST", "/api/neighbors", HandleAPINeighborsExt)
	route("POST", "/api/ips_info", HandleAPIIPsInfo)
	route("POST", "/api/ips_sanctions", HandleAPIIPsSanctions)
	route("POST", "/api/user_nodes", WithUser, HandleAPISetUserNode)
	route("DELETE", "/api/user_nodes", WithUser, HandleAPIDelUserNode)
	route("GET", "/api/sat_nodes", HandleAPIGetSatNodes)
	route("GET", "/api/user_nodes/my/:node_id/pings", WithUser, WithGzip, HandleAPIUserNodePings)
	route("GET", "/api/user_nodes/sat/:node_id/pings", WithGzip, HandleAPIUserNodePings)
	route("POST", "/api/user_texts", WithUser, HandleAPIUserTexts)
	route("GET", "/api/storj_token/summary", WithGzip, HandleAPIStorjTokenTxSummary)
	route("GET", "/api/nodes/locations", WithGzip, HandleAPINodesLocations)
	route("GET", "/api/nodes/location_summary", HandleAPINodesLocationSummary)
	route("GET", "/api/nodes/countries", WithGzip, HandleAPINodesCountries)
	route("GET", "/api/nodes/subnet_summary", HandleAPINodesSubnetSummary)
	route("GET", "/api/nodes/counts", WithGzip, HandleAPINodesCounts)
	route("POST", "/api/client_errors", WithOptUser, HandleAPIClientErrors)

	route("GET", "/api/explode", func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
		return nil, merry.New("test API error")
	})
	route("GET", "/explode", func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
		return merry.New("test error")
	})

	wrapped404 := wrapper.WrapChain(Handle404)
	router.NotFound = http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		wrapped404(wr, r, httprouter.Params{{}})
	})

	if env.IsDev() {
		devServerAddress, err := httputils.RunBundleDevServerNear(address, baseDir+"/www", "--configHost", "--configPort")
		if err != nil {
			return merry.Wrap(err)
		}
		bundleFPath = "http://" + devServerAddress + "/bundle.js"
		stylesFPath = "http://" + devServerAddress + "/bundle.css"
	} else {
		distPath := baseDir + "/www/dist"
		bundleFPath, stylesFPath, err = httputils.LastJSAndCSSFNames(distPath, "bundle.", "bundle.")
		if err != nil {
			return merry.Wrap(err)
		}
		bundleFPath = "/dist/" + bundleFPath
		stylesFPath = "/dist/" + stylesFPath
		// router.ServeFiles("/dist/*filepath", http.Dir(distPath))
	}
	log.Info().Str("fpath", bundleFPath).Msg("bundle")
	log.Info().Str("fpath", stylesFPath).Msg("styles")

	// Server
	log.Info().Msg("starting server on " + address)
	return merry.Wrap(http.ListenAndServe(address, router))
}
