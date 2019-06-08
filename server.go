package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
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

func sizeIB(size int64) string {
	i := 0
	for ; i < len(sizeIBNames)-1 && 1<<uint(i*10+10) < size; i++ {
	}
	return strconv.FormatFloat(float64(size)/float64(int(1)<<uint(i*10)), 'g', 3, 64) + " " + sizeIBNames[i]
}

var templateFuncs = template.FuncMap{
	"loc": func(lang string, en string, special ...string) string {
		if lang == "en" {
			return en
		}
		for i := 0; i < len(special)-1; i += 2 {
			if lang == special[i] {
				return special[i+1]
			}
		}
		return en
	},
	"formatDateTime": func(t time.Time, lang string) string {
		switch lang {
		case "ru":
			return t.Format("02.01.2006 15:04")
		default:
			return t.Format("1/2/2006 15:04")
		}
	},
	"ago": func(t time.Time, lang string) string {
		delta := time.Now().Sub(t)
		days := int64(delta / (24 * time.Hour))
		hours := int64((delta / time.Hour) % 24)
		minutes := int64((delta / time.Minute) % 60)
		switch lang {
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
	},
	"pluralize": func(valI interface{}, lang string, words ...string) string {
		var val int64
		switch v := valI.(type) {
		case int:
			val = int64(v)
		default:
			val = v.(int64)
		}
		return pluralize(val, lang, words...)
	},
	"sub": func(a, b int64) int64 {
		return a - b
	},
	"div": func(a, b int64) int64 {
		return a / b
	},
	"signed": func(a int64) string {
		res := strconv.FormatInt(a, 10)
		if a > 0 {
			res = "+" + res
		}
		return res
	},
	"sizeIB": sizeIB,
	"sizeIBSign": func(size int64) string {
		if size > 0 {
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

func getTemplate(path string) (*template.Template, error) {
	if true {
		if tmpl, ok := templateCache[path]; ok {
			return tmpl, nil
		}
	}

	log.Println("building template: " + path)
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

func render(wr http.ResponseWriter, statusCode int, tmplName, blockName string, data interface{}) error {
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
		r = r.WithContext(context.WithValue(r.Context(), ctxKey("db"), db))
		if err := handle(wr, r, ps); err != nil {
			d := merry.Details(err)
			log.Print(d)
			//Handle500(wr, r, ps)
			wr.Write([]byte("<pre>" + d))
		}
	}
}

func Handle404(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	return render(wr, http.StatusNotFound, "404.html", "404.html", nil)
}

func Handle500(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	return render(wr, http.StatusInternalServerError, "500.html", "500.html", nil)
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

	orderedTypes := []pb.NodeType{pb.NodeType_SATELLITE, pb.NodeType_BOOTSTRAP} //pb.NodeType_UPLINK, pb.NodeType_INVALID, pb.NodeType_STORAGE
	var nodeIDsWithType []struct {
		NodeType pb.NodeType
		IDs      []NodeIDExt `sql:"ids,array"`
	}
	_, err = db.Query(&nodeIDsWithType, `
		SELECT node_type, ids FROM (
			SELECT (self_params->'type')::int AS node_type, array_agg(id ORDER BY id) AS ids
			FROM nodes
			WHERE self_updated_at > NOW() - INTERVAL '24 hours'
			AND self_params->'type' IS NOT NULL AND (self_params->'type')::int != ?
			GROUP BY (self_params->'type')::int
		) AS tt
		JOIN unnest(?::int[]) WITH ORDINALITY AS t(node_type, ord) USING(node_type) ORDER BY t.ord
		`, pb.NodeType_STORAGE, pg.Array(orderedTypes))
	if err != nil {
		return merry.Wrap(err)
	}

	return render(wr, http.StatusOK, "index.html", "base", map[string]interface{}{
		"Lang":             "ru",
		"LastStat":         lastStat,
		"DayAgoStat":       dayAgoStat,
		"NodeIDsWithType":  nodeIDsWithType,
		"NodeType_STORAGE": pb.NodeType_STORAGE,
	})
}

func HandleNode(wr http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	db := r.Context().Value(ctxKey("db")).(*pg.DB)

	nodeIDStr := ps.ByName("nodeID")
	nodeID, err := storj.NodeIDFromString(nodeIDStr)
	if err != nil {
		return render(wr, http.StatusBadRequest, "node.html", "base", map[string]interface{}{
			"Lang":            "ru",
			"NodeIDStr":       nodeIDStr,
			"NodeIDIsInvalid": true,
			"NodeIDDecodeErr": err.Error(),
		})
	}

	node := &Node{ID: nodeID}
	err = db.Model(node).Column("created_at", "kad_params", "kad_updated_at", "self_updated_at", "self_params", "location").Where("id = ?", nodeID).Select()
	if err == pg.ErrNoRows {
		return render(wr, http.StatusBadRequest, "node.html", "base", map[string]interface{}{
			"Lang":           "ru",
			"NodeIDStr":      node.ID.String(),
			"NodeIDNotFound": true,
			"Node":           node,
		})
	}
	if err != nil {
		return merry.Wrap(err)
	}

	return render(wr, http.StatusOK, "node.html", "base", map[string]interface{}{
		"Lang":      "ru",
		"NodeIDStr": node.ID.String(),
		"Node":      node,
	})
}

func StartHTTPServer(address string) error {
	db := makePGConnection()

	router := httprouter.New()
	router.Handle("GET", "/", wrap(db, HandleIndex))
	router.Handle("GET", "/@:nodeID", wrap(db, HandleNode))

	jsFS := http.FileServer(http.Dir("www/js"))
	router.Handler("GET", "/js/*fpath", http.StripPrefix("/js/", jsFS))

	wrapped404 := wrap(db, Handle404)
	router.NotFound = http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		wrapped404(wr, r, httprouter.Params{{}})
	})
	return merry.Wrap(http.ListenAndServe(address, router))
}
