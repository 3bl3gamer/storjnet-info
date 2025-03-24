package storjutils

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog/log"
	"storj.io/common/storj"
)

type PingResult struct {
	PingDurations
	Error string `json:"error"`
}

const PingUDPProxyPacketRepeat = 3

type PingUDPProxyRequest struct {
	RequestID uint32
	ID        string
	Addr      string
	Mode      string
	Timeout   int64
	DialOnly  bool
}
type PingUDPProxyResponse struct {
	RequestID uint32
	PingResult
}

func StartPingProxy(address, identityDirPath, proxyMode string) error {
	endpointPath, ok := os.LookupEnv("PROXY_ENDPOINT_PATH")
	if !ok {
		return merry.New("PROXY_ENDPOINT_PATH env variable is required")
	}

	sat := &SatelliteLocal{}
	if err := sat.SetUp("Local", identityDirPath); err != nil {
		return merry.Wrap(err)
	}

	if proxyMode == "http" {
		return StartPingHTTPProxy(address, endpointPath, sat)
	} else if proxyMode == "udp" {
		return StartPingUDPProxy(address, endpointPath, sat)
	} else {
		return merry.Errorf("invalid proxy mode: %s", proxyMode)
	}
}

func StartPingHTTPProxy(address, endpointPath string, sat *SatelliteLocal) error {

	http.HandleFunc(endpointPath, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		nodeAddr := query.Get("addr")

		nodeIDStr := query.Get("id")
		nodeID, err := storj.NodeIDFromString(nodeIDStr)
		if err != nil {
			log.Error().Err(err).Str("raw", nodeIDStr).Msg("HTTP: invalid node ID")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		modeStr := query.Get("mode")
		var mode SatMode
		if modeStr == "tcp" {
			mode = SatModeTCP
		} else if modeStr == "quic" {
			mode = SatModeQUIC
		} else {
			log.Error().Err(err).Str("raw", modeStr).Msg("HTTP: invalid mode")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		timeoutStr := query.Get("timeout")
		timeoutMS, err := strconv.Atoi(timeoutStr)
		if err != nil || timeoutMS <= 0 || timeoutMS > 60*1000 {
			log.Error().Err(err).Str("raw", timeoutStr).Msg("HTTP: invalid timeout")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		dialOnly := query.Get("dialOnly") == "1"

		durs, err := sat.PingAndClose(nodeAddr, nodeID, mode, dialOnly, time.Duration(timeoutMS)*time.Millisecond)
		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		json.NewEncoder(w).Encode(PingResult{PingDurations: durs, Error: errStr})
	})

	log.Info().Msg("HTTP: starting server on " + address)
	return merry.Wrap(http.ListenAndServe(address, nil))
}

func StartPingUDPProxy(address, endpointPath string, sat *SatelliteLocal) error {
	ipStr, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return merry.Wrap(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return merry.Wrap(err)
	}
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP(ipStr),
	}

	ser, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return merry.Wrap(err)
	}

	log.Info().Msg("UDP: starting server on " + address)
	lastRequestIDs := make([]uint32, 64)
	for {
		inBuf := make([]byte, 256)
		n, remoteaddr, err := ser.ReadFromUDP(inBuf)
		if err != nil {
			log.Error().Err(err).Msg("UDP: reading socket")
			continue
		}

		payload, endpointMatches := DecodePingUDPProxyMaskedBuf(inBuf[:n], endpointPath)
		if !endpointMatches {
			log.Error().Msg("UDP: invalid payload prefix")
			continue
		}

		var data PingUDPProxyRequest
		if err := json.Unmarshal(payload, &data); err != nil {
			log.Error().Err(err).Msg("UDP: invalid payload data")
			continue
		}

		if slices.Contains(lastRequestIDs, data.RequestID) {
			continue //duplicate
		}
		for i := 1; i < len(lastRequestIDs); i++ {
			lastRequestIDs[i-1] = lastRequestIDs[i]
		}
		lastRequestIDs[len(lastRequestIDs)-1] = data.RequestID

		go func() {
			nodeID, err := storj.NodeIDFromString(data.ID)
			if err != nil {
				log.Error().Err(err).Str("raw", data.ID).Msg("UDP: invalid node ID")
				return
			}

			var mode SatMode
			if data.Mode == "tcp" {
				mode = SatModeTCP
			} else if data.Mode == "quic" {
				mode = SatModeQUIC
			} else {
				log.Error().Err(err).Str("raw", data.Mode).Msg("UDP: invalid mode")
				return
			}

			if data.Timeout <= 0 || data.Timeout > 60*1000 {
				log.Error().Err(err).Int64("raw", data.Timeout).Msg("UDP: invalid timeout")
				return
			}

			durs, err := sat.PingAndClose(data.Addr, nodeID, mode, data.DialOnly, time.Duration(data.Timeout)*time.Millisecond)
			var errStr string
			if err != nil {
				errStr = err.Error()
			}

			respPayload, err := json.Marshal(PingUDPProxyResponse{
				RequestID:  data.RequestID,
				PingResult: PingResult{PingDurations: durs, Error: errStr},
			})
			if err != nil {
				log.Error().Err(err).Msg("UDP: JSON encode error")
				return
			}
			outBuf := EncodePingUDPProxyMaskedBuf(respPayload, endpointPath)

			for i := 0; i < PingUDPProxyPacketRepeat; i++ {
				if _, err := ser.WriteToUDP(outBuf, remoteaddr); err != nil {
					log.Error().Err(err).Int("iteration", i).Msg("UDP: write error")
				}
			}
		}()
	}
}

func DecodePingUDPProxyMaskedBuf(buf []byte, endpointPath string) ([]byte, bool) {
	mask := buf[0:4]
	payload := buf[4:]
	for i := range payload {
		payload[i] ^= mask[i%len(mask)]
	}
	if !bytes.HasPrefix(payload, []byte(endpointPath)) {
		return nil, false
	}
	return payload[len(endpointPath):], true
}

func EncodePingUDPProxyMaskedBuf(payload []byte, endpointPath string) []byte {
	mask := rand.Uint32()
	buf := make([]byte, 4+len(endpointPath)+len(payload))
	binary.LittleEndian.PutUint32(buf, mask)
	copy(buf[4:], []byte(endpointPath))
	copy(buf[4+len(endpointPath):], payload)

	maskBuf := buf[:4]
	outPayload := buf[4:]
	for i := range outPayload {
		outPayload[i] ^= maskBuf[i%len(maskBuf)]
	}
	return buf
}
