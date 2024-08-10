package utils

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/quic"
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

type Satellite struct {
	Label      string
	Config     satellite.Config
	TCPDialer  rpc.Dialer
	QUICDialer *rpc.Dialer
}

func (sat *Satellite) SetUp(label string, identityDir string, tcpProxyDialer proxy.ContextDialer) error {
	sat.Label = label

	sat.Config.Identity.CertPath = identityDir + "/identity.cert"
	sat.Config.Identity.KeyPath = identityDir + "/identity.key"
	sat.Config.Server.Config.PeerIDVersions = "*"
	identity, err := sat.Config.Identity.Load()
	if err != nil {
		return merry.Wrap(err)
	}
	tlsOptions, err := tlsopts.NewOptions(identity, sat.Config.Server.Config, nil) //revocationDB
	if err != nil {
		return merry.Wrap(err)
	}

	var tcpDialer rpc.DialFunc
	if tcpProxyDialer != nil {
		tcpDialer = tcpProxyDialer.DialContext
	}
	sat.TCPDialer = rpc.NewDefaultDialer(tlsOptions)
	sat.TCPDialer.Connector = rpc.NewDefaultTCPConnector(tcpDialer)

	if tcpProxyDialer == nil {
		quicDialer := rpc.NewDefaultDialer(tlsOptions)
		sat.QUICDialer = &quicDialer
		sat.QUICDialer.Connector = quic.NewDefaultConnector(nil)
	}
	return nil
}

func (sat *Satellite) dialerFor(mode SatMode) *rpc.Dialer {
	if mode == SatModeTCP {
		return &sat.TCPDialer
	}
	return sat.QUICDialer
}

func (sat *Satellite) UsesProxy() bool {
	return sat.QUICDialer == nil
}

func (sat *Satellite) Dial(ctx context.Context, address string, id storj.NodeID, mode SatMode) (*rpc.Conn, error) {
	dialer := sat.dialerFor(mode)
	if dialer == nil {
		return nil, merry.Errorf("can not dial satellite '%s' via QUIC")
	}
	conn, err := dialer.DialNodeURL(ctx, storj.NodeURL{Address: address, ID: id})
	if err != nil {
		return nil, merry.Wrap(err)
	}
	// forcing Dial to happen NOW (otherwise it will be delayed until next RPC call)
	if err := conn.ForceState(ctx); err != nil {
		return nil, merry.Wrap(err)
	}
	return conn, nil
}

func (sat *Satellite) DialAndClose(address string, id storj.NodeID, mode SatMode, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := sat.Dial(ctx, address, id, mode)
	if err != nil {
		return merry.Wrap(err)
	}
	return merry.Wrap(conn.Close())
}

func (sat *Satellite) Ping(ctx context.Context, conn *rpc.Conn, mode SatMode) error {
	client := pb.NewDRPCContactClient(conn)
	_, err := client.PingNode(ctx, &pb.ContactPingRequest{})
	return merry.Wrap(err)
}

type Satellites []*Satellite

const SatsEnvCfgKey = "SATELLITES"

func SatellitesSetUpFromEnv() (Satellites, error) {
	value, ok := os.LookupEnv(SatsEnvCfgKey)
	if !ok {
		log.Warn().Msgf("no '%s' env key, using default local satellite", SatsEnvCfgKey)
		sat := &Satellite{}
		if err := sat.SetUp("Local", "identity", nil); err != nil {
			return nil, merry.Wrap(err)
		}
		return []*Satellite{sat}, nil
	}

	var sats Satellites
	items := strings.Split(value, "|")
	for _, item := range items {
		parts := strings.Split(item, ":")
		sat := &Satellite{}

		if len(parts) == 2 {
			// label:path/to/identity
			if err := sat.SetUp(parts[0], parts[1], nil); err != nil {
				return nil, merry.Wrap(err)
			}
		} else if len(parts) == 4 || len(parts) == 6 {
			// label:path/to/identity:proxyHost:port
			// label:path/to/identity:proxyHost:port:username:password
			var auth *proxy.Auth
			if len(parts) == 6 {
				auth = &proxy.Auth{User: parts[4], Password: parts[5]}
			}
			dialer, err := proxy.SOCKS5("tcp", parts[2]+":"+parts[3], auth, proxy.Direct)
			if err != nil {
				return nil, merry.Wrap(err)
			}
			// SOCKS5() returns proxy.Dialer which in fact is also a proxy.ContextDialer
			ctxDialer := dialer.(proxy.ContextDialer)
			if err := sat.SetUp(parts[0], parts[1], ctxDialer); err != nil {
				return nil, merry.Wrap(err)
			}
		} else {
			return nil, merry.Errorf("wrong satellite description '%s', expected"+
				" label:path/to/identity or label:proxyHost:port or label:proxyHost:port:username:password", item)
		}

		sats = append(sats, sat)
	}
	return sats, nil
}

func (sats Satellites) DialAndClose(address string, id storj.NodeID, mode SatMode, timeout time.Duration) (*Satellite, error) {
	var lastErr error
	for _, sat := range sats {
		lastErr = sat.DialAndClose(address, id, mode, timeout)
		if lastErr == nil {
			return sat, nil
		}
	}
	return nil, merry.Wrap(lastErr)
}
