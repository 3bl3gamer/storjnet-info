package utils

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog/log"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/satellite"
)

type SatMode int

const (
	SatModeTCP SatMode = iota
	SatModeQUIC
)

// Since v1.30.2 nodes return error to pings from untrusted satellites
// https://github.com/storj/storj/releases/tag/v1.30.2
func IsUntrustedSatPingError(err error) bool {
	s := err.Error()
	return strings.HasPrefix(s, "trust: satellite ") && strings.HasSuffix(s, " is untrusted")
}

type PingDurations struct {
	DialDuration float64 `json:"dial_duration"`
	PingDuration float64 `json:"ping_duration"`
}
type PingResult struct {
	PingDurations
	Error string `json:"error"`
}

type Satellite interface {
	Label() string
	UsesProxy() bool
	PingAndClose(address string, id storj.NodeID, mode SatMode, dialOnly bool, timeout time.Duration) (PingDurations, error)
}

type SatelliteLocal struct {
	label      string
	config     satellite.Config
	tcpDialer  rpc.Dialer
	quicDialer rpc.Dialer
}

func (sat *SatelliteLocal) SetUp(label string, identityDir string) error {
	sat.label = label

	sat.config.Identity.CertPath = identityDir + "/identity.cert"
	sat.config.Identity.KeyPath = identityDir + "/identity.key"
	sat.config.Server.Config.PeerIDVersions = "*"
	identity, err := sat.config.Identity.Load()
	if err != nil {
		return merry.Wrap(err)
	}
	tlsOptions, err := tlsopts.NewOptions(identity, sat.config.Server.Config, nil) //revocationDB
	if err != nil {
		return merry.Wrap(err)
	}

	sat.tcpDialer = rpc.NewDefaultDialer(tlsOptions)
	// sat.TCPDialer.Connector = rpc.NewDefaultTCPConnector(nil)
	sat.quicDialer = rpc.NewDefaultDialer(tlsOptions)
	// sat.QUICDialer.Connector = quic.NewDefaultConnector(nil)
	return nil
}

func (sat *SatelliteLocal) dialerFor(mode SatMode) rpc.Dialer {
	if mode == SatModeTCP {
		return sat.tcpDialer
	}
	return sat.quicDialer
}

func (sat *SatelliteLocal) dial(ctx context.Context, address string, id storj.NodeID, mode SatMode) (*rpc.Conn, error) {
	conn, err := sat.dialerFor(mode).DialNodeURL(ctx, storj.NodeURL{Address: address, ID: id})
	if err != nil {
		return nil, merry.Wrap(err)
	}
	// forcing Dial to happen NOW (otherwise it will be delayed until next RPC call)
	if err := conn.ForceState(ctx); err != nil {
		return nil, merry.Wrap(err)
	}
	return conn, nil
}

func (sat *SatelliteLocal) ping(ctx context.Context, conn *rpc.Conn) error {
	client := pb.NewDRPCContactClient(conn)
	_, err := client.PingNode(ctx, &pb.ContactPingRequest{})
	return merry.Wrap(err)
}

func (sat *SatelliteLocal) Label() string {
	return sat.label
}

func (sat *SatelliteLocal) UsesProxy() bool {
	return false
}

func (sat *SatelliteLocal) PingAndClose(address string, id storj.NodeID, mode SatMode, dialOnly bool, timeout time.Duration) (PingDurations, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var dialDuration, pingDuration float64

	stt := time.Now()
	conn, err := sat.dial(ctx, address, id, mode)
	dialDuration = time.Since(stt).Seconds()
	if err != nil {
		return PingDurations{DialDuration: dialDuration, PingDuration: pingDuration}, err
	}
	defer conn.Close()

	if !dialOnly {
		stt := time.Now()
		err := sat.ping(ctx, conn)
		pingDuration = time.Since(stt).Seconds()
		if err != nil && !IsUntrustedSatPingError(err) {
			return PingDurations{DialDuration: dialDuration, PingDuration: pingDuration}, err
		}
	}

	return PingDurations{DialDuration: dialDuration, PingDuration: pingDuration}, nil
}

type SatelliteProxy struct {
	label   string
	address string
	path    string
	client  *http.Client
}

func (sat *SatelliteProxy) SetUp(label, address, path string) error {
	sat.label = label
	sat.address = address
	sat.path = path
	sat.client = &http.Client{}
	return nil
}

func (sat *SatelliteProxy) Label() string {
	return sat.label
}

func (sat *SatelliteProxy) UsesProxy() bool {
	return true
}

func (sat *SatelliteProxy) PingAndClose(address string, id storj.NodeID, mode SatMode, dialOnly bool, timeout time.Duration) (PingDurations, error) {
	modeStr := "tcp"
	if mode == SatModeQUIC {
		modeStr = "quic"
	}

	query := make(url.Values)
	query.Set("id", id.String())
	query.Set("addr", address)
	query.Set("mode", modeStr)
	query.Set("timeout", strconv.FormatInt(int64(timeout/time.Millisecond), 10))
	if dialOnly {
		query.Set("dialOnly", "1")
	}

	resp, err := sat.client.Post("http://"+sat.address+sat.path+"?"+query.Encode(), "text/plain", nil)
	if err != nil {
		return PingDurations{}, merry.New("proxy request error") //should not reveal full error message with full path
	}
	defer resp.Body.Close()

	var pingRes PingResult
	if err := json.NewDecoder(resp.Body).Decode(&pingRes); err != nil {
		return PingDurations{}, merry.New("proxy response error")
	}

	err = nil
	if pingRes.Error != "" {
		err = errors.New(pingRes.Error)
	}
	return pingRes.PingDurations, err
}

type Satellites []Satellite

const SatsEnvCfgKey = "SATELLITES"

func SatellitesSetUpFromEnv() (Satellites, error) {
	value, ok := os.LookupEnv(SatsEnvCfgKey)
	if !ok {
		log.Warn().Msgf("no '%s' env key, using default local satellite", SatsEnvCfgKey)
		sat := &SatelliteLocal{}
		if err := sat.SetUp("Local", "identity"); err != nil {
			return nil, merry.Wrap(err)
		}
		return []Satellite{sat}, nil
	}

	var sats Satellites
	items := strings.Split(value, "|")
	for _, item := range items {
		parts := strings.SplitN(item, ":", 4)

		if len(parts) == 2 {
			// label:path/to/identity
			sat := &SatelliteLocal{}
			if err := sat.SetUp(parts[0], parts[1]); err != nil {
				return nil, merry.Wrap(err)
			}
			sats = append(sats, sat)
		} else if len(parts) == 4 {
			// label:/path:port:ip
			sat := &SatelliteProxy{}
			sat.SetUp(parts[0], parts[3]+":"+parts[2], parts[1])
			sats = append(sats, sat)
		} else {
			return nil, merry.Errorf("wrong satellite description '%s', expected label:path/to/identity or label:/path:port:ip", item)
		}
	}
	return sats, nil
}

func (sats Satellites) DialAndClose(address string, id storj.NodeID, mode SatMode, timeout time.Duration) (Satellite, error) {
	var lastErr error
	for _, sat := range sats {
		_, lastErr = sat.PingAndClose(address, id, mode, true, timeout)
		if lastErr == nil {
			return sat, nil
		}
	}
	return nil, merry.Wrap(lastErr)
}
