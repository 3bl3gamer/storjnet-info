package utils

import (
	"context"

	"github.com/ansel1/merry"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/satellite"
)

type Satellite struct {
	Config satellite.Config
	Dialer rpc.Dialer
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

	sat.Dialer = rpc.NewDefaultDialer(tlsOptions)
	return nil
}

func (sat *Satellite) Dial(ctx context.Context, address string, id storj.NodeID) (*rpc.Conn, error) {
	conn, err := sat.Dialer.DialNodeURL(ctx, storj.NodeURL{Address: address, ID: id})
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
	client := pb.NewDRPCContactClient(conn)
	_, err := client.PingNode(ctx, &pb.ContactPingRequest{})
	return merry.Wrap(err)
}
