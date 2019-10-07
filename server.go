package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"storj3stat/utils"
	"time"

	httputils "github.com/3bl3gamer/go-http-utils"
	"github.com/ansel1/merry"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
	"storj.io/storj/pkg/storj"
)

type ctxKey string

const CtxKeySatellite = ctxKey("satellite")

func unmarshalFromBody(r *http.Request, obj interface{}) *httputils.JsonError {
	if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
		descr := ""
		if envMode == "dev" {
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
	"en": []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"},
	"ru": []string{"январь", "февраль", "март", "апрель", "май", "июнь", "июль", "август", "сентябрь", "октябрь", "ноябрь", "декабрь"},
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

func HandleIndex(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	return map[string]interface{}{"FPath": "index.html"}, nil
}

func HandlePingMyNode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (httputils.TemplateCtx, error) {
	return map[string]interface{}{"FPath": "ping_my_node.html"}, nil
}

func HandlePingMyNodeAPI(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
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

	stt := time.Now()
	conn, err := sat.Dial(params.Address, id)
	if err != nil {
		return httputils.JsonError{Code: 400, Error: "NODE_DIAL_ERROR", Description: err.Error()}, nil
	}
	dialDuration := time.Now().Sub(stt).Seconds()
	defer conn.Close()

	var pingDuration float64
	if !params.DialOnly {
		stt := time.Now()
		if err := sat.Ping(conn); err != nil {
			return httputils.JsonError{Code: 400, Error: "NODE_PING_ERROR", Description: err.Error()}, nil
		}
		pingDuration = time.Now().Sub(stt).Seconds()
	}
	return map[string]interface{}{"pingDuration": pingDuration, "dialDuration": dialDuration}, nil
}

func HandleHtml500(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	tmplHnd := r.Context().Value(httputils.CtxKeyMain).(*httputils.MainCtx).TemplateHandler
	return merry.Wrap(tmplHnd.RenderTemplate(wr, r, map[string]interface{}{"FPath": "500.html"}))
}

func StartHTTPServer(address string) error {
	ex, err := os.Executable()
	if err != nil {
		return merry.Wrap(err)
	}
	baseDir := filepath.Dir(ex)

	sat := &utils.Satellite{}
	if err := sat.SetUp(); err != nil {
		return merry.Wrap(err)
	}

	wrapper := &httputils.Wrapper{
		ShowErrorDetails: envMode == "dev",
		ExtraChainItem: func(handle httputils.HandlerExt) httputils.HandlerExt {
			return func(wr http.ResponseWriter, r *http.Request, params httprouter.Params) error {
				log.Debug().Str("method", r.Method).Str("path", r.URL.Path).Msg("request")
				r = r.WithContext(context.WithValue(r.Context(), CtxKeySatellite, sat))
				return merry.Wrap(handle(wr, r, params))
			}
		},
		TemplateHandler: &httputils.TemplateHandler{
			CacheParsed: envMode == "prod",
			BasePath:    baseDir + "/www/templates",
			FuncMap:     template.FuncMap{},
			ParamsFunc: func(r *http.Request, ctx *httputils.MainCtx, params httputils.TemplateCtx) error {
				params["L"] = L10nUtls{"ru"}
				return nil
			},
			LogBuild: func(path string) { log.Info().Str("path", path).Msg("building template") },
		},
		HandleHtml500: HandleHtml500,
		LogError: func(err error, r *http.Request) {
			log.Error().Stack().Err(err).Str("method", r.Method).Str("path", r.URL.Path).Msg("")
		},
	}

	router := httprouter.New()
	route := func(method, path string, chain ...interface{}) {
		router.Handle(method, path, wrapper.WrapChain(chain...))
	}

	// Routes
	route("GET", "/", HandleIndex)
	route("GET", "/ping_my_node", HandlePingMyNode)
	route("POST", "/api/ping_my_node", HandlePingMyNodeAPI)
	route("GET", "/api/explode", func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) (interface{}, error) {
		return nil, merry.New("test API error")
	})
	route("GET", "/explode", func(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
		return merry.New("test error")
	})
	router.ServeFiles("/js/*filepath", http.Dir(baseDir+"/www/js"))
	router.ServeFiles("/css/*filepath", http.Dir(baseDir+"/www/css"))

	log.Info().Msg("starting server on " + address)
	return merry.Wrap(http.ListenAndServe(address, router))
}
