package utils

import (
	"context"
	"strings"

	"github.com/ansel1/merry"
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
	Config     satellite.Config
	TCPDialer  rpc.Dialer
	QUICDialer rpc.Dialer
}

func (sat *Satellite) SetUp(identityDir string) error {
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

	sat.TCPDialer = rpc.NewDefaultDialer(tlsOptions)
	sat.QUICDialer = rpc.NewDefaultDialer(tlsOptions)
	sat.QUICDialer.Connector = quic.NewDefaultConnector(nil)
	return nil
}

func (sat *Satellite) dialerFor(mode SatMode) rpc.Dialer {
	if mode == SatModeTCP {
		return sat.TCPDialer
	}
	return sat.QUICDialer
}

func (sat *Satellite) Dial(ctx context.Context, address string, id storj.NodeID, mode SatMode) (*rpc.Conn, error) {
	conn, err := sat.dialerFor(mode).DialNodeURL(ctx, storj.NodeURL{Address: address, ID: id})
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return conn, nil
}

func (sat *Satellite) DialAndClose(ctx context.Context, address string, id storj.NodeID, mode SatMode) error {
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
