package utils

import (
	"context"

	"github.com/ansel1/merry"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
)

type Satellite struct {
	Config satellite.Config
	Dialer rpc.Dialer
}

func (sat *Satellite) SetUp() error {
	sat.Config.Identity.CertPath = "identity/identity.cert"
	sat.Config.Identity.KeyPath = "identity/identity.key"
	sat.Config.Server.Config.PeerIDVersions = "*"
	identity, err := sat.Config.Identity.Load()
	if err != nil {
		return merry.Wrap(err)
	}
	tlsOptions, err := tlsopts.NewOptions(identity, sat.Config.Server.Config, nil) //revocationDB
	if err != nil {
		return merry.Wrap(err)
	}

	sat.Dialer = rpc.NewDefaultDialer(tlsOptions)
	return nil
}

func (sat *Satellite) Dial(ctx context.Context, address string, id storj.NodeID) (*rpc.Conn, error) {
	conn, err := sat.Dialer.DialAddressID(ctx, address, id)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return conn, nil
}

func (sat *Satellite) DialAndClose(ctx context.Context, address string, id storj.NodeID) error {
	conn, err := sat.Dial(ctx, address, id)
	if err != nil {
		return merry.Wrap(err)
	}
	return merry.Wrap(conn.Close())
}

func (sat *Satellite) Ping(ctx context.Context, conn *rpc.Conn) error {
	client := conn.ContactClient()
	_, err := client.PingNode(ctx, &pb.ContactPingRequest{})
	return merry.Wrap(err)
}
