package proxy

import (
	"encoding/json"
	"net/http"
	"os"
	"storjnet/utils"
	"strconv"
	"time"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog/log"
	"storj.io/common/storj"
)

func StartPingProxy(address, identityDirPath string) error {
	path, ok := os.LookupEnv("PROXY_ENDPOINT_PATH")
	if !ok {
		return merry.New("PROXY_ENDPOINT_PATH env variable is required")
	}

	sat := &utils.SatelliteLocal{}
	if err := sat.SetUp("Local", identityDirPath); err != nil {
		return merry.Wrap(err)
	}

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		nodeAddr := query.Get("addr")

		nodeIDStr := query.Get("id")
		nodeID, err := storj.NodeIDFromString(nodeIDStr)
		if err != nil {
			log.Error().Err(err).Str("raw", nodeIDStr).Msg("invalid node ID")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		modeStr := query.Get("mode")
		var mode utils.SatMode
		if modeStr == "tcp" {
			mode = utils.SatModeTCP
		} else if modeStr == "quic" {
			mode = utils.SatModeQUIC
		} else {
			log.Error().Err(err).Str("raw", modeStr).Msg("invalid mode")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		timeoutStr := query.Get("timeout")
		timeoutMS, err := strconv.Atoi(timeoutStr)
		if err != nil || timeoutMS <= 0 || timeoutMS > 60*1000 {
			log.Error().Err(err).Str("raw", timeoutStr).Msg("invalid timeout")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		dialOnly := query.Get("dialOnly") == "1"

		durs, err := sat.PingAndClose(nodeAddr, nodeID, mode, dialOnly, time.Duration(timeoutMS)*time.Millisecond)
		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		json.NewEncoder(w).Encode(utils.PingResult{PingDurations: durs, Error: errStr})
	})

	log.Info().Msg("starting server on " + address)
	return merry.Wrap(http.ListenAndServe(address, nil))
}
