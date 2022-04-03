package nodes

import (
	"context"
	"storjnet/utils"
	"strconv"
	"strings"
	"time"

	"github.com/abh/geoip"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/uplink/private/metaclient"
)

func FetchAndProcess(satelliteAddress string) error {
	apiKey, err := utils.RequireEnv("STORJ_API_KEY")
	if err != nil {
		return merry.Wrap(err)
	}

	db := utils.MakePGConnection()
	gdb, err := utils.MakeGeoIPConnection()
	if err != nil {
		return merry.Wrap(err)
	}
	ctx := context.Background()

	parsedAPIKey, err := macaroon.ParseAPIKey(apiKey)
	if err != nil {
		return merry.Wrap(err)
	}
	metainfoClient, _, _, err := dial(ctx, satelliteAddress, parsedAPIKey)
	if err != nil {
		return merry.Wrap(err)
	}

	beginObjectReq := &metaclient.BeginObjectParams{
		Bucket:             []byte("test-bucket"),
		EncryptedObjectKey: []byte("f1"),
		ExpiresAt:          time.Now().Add(time.Minute),
	}
	maxEncryptedSegmentSize := int64(67254016)
	currentSegment := 0
	beginSegment := metaclient.BeginSegmentParams{
		MaxOrderLimit: maxEncryptedSegmentSize,
		Position: storj.SegmentPosition{
			Index: int32(currentSegment),
		},
	}
	responses, err := metainfoClient.Batch(ctx, beginObjectReq, &beginSegment)
	if err != nil {
		return merry.Wrap(err)
	}
	segResponse, err := responses[1].BeginSegment()
	if err != nil {
		return merry.Wrap(err)
	}
	return merry.Wrap(saveLimits(db, gdb, satelliteAddress, segResponse.Limits))
}

type NodeLocation struct {
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Longitude float32 `json:"longitude"`
	Latitude  float32 `json:"latitude"`
}

func saveLimits(db *pg.DB, gdb *geoip.GeoIP, satelliteAddress string, limits []*pb.AddressedOrderLimit) error {
	stt := time.Now()
	newCount := 0
	locCount := 0
	err := db.RunInTransaction(func(tx *pg.Tx) error {
		ids := make([]storj.NodeID, len(limits))
		for i, l := range limits {
			ids[i] = l.Limit.StorageNodeId
		}
		_, err := tx.Exec(`SELECT 1 FROM storjnet.nodes WHERE id IN (?) FOR UPDATE`, pg.In(ids))
		if err != nil {
			return merry.Wrap(err)
		}
		_, err = tx.Exec(`SELECT 1 FROM storjnet.nodes_sat_offers WHERE node_id IN (?) FOR UPDATE`, pg.In(ids))
		if err != nil {
			return merry.Wrap(err)
		}

		for _, l := range limits {
			nodeID := l.Limit.StorageNodeId
			addr := l.StorageNodeAddress.Address
			index := strings.LastIndexByte(addr, ':')
			if index < 0 {
				return merry.New("no ip:port in " + addr)
			}
			ipAddr := addr[:index]
			port, err := strconv.Atoi(addr[index+1:])
			if err != nil {
				return merry.Wrap(err)
			}

			var loc *NodeLocation
			if rec := gdb.GetRecord(ipAddr); rec != nil {
				loc = &NodeLocation{
					Country:   rec.CountryName,
					City:      rec.City,
					Longitude: rec.Longitude,
					Latitude:  rec.Latitude,
				}
			}

			var xmax string
			_, err = tx.QueryOne(&xmax, `
				INSERT INTO storjnet.nodes
					(id, ip_addr, port, location, last_received_from_sat_at) VALUES (?,?,?,?,NOW())
				ON CONFLICT (id) DO UPDATE SET
					ip_addr = EXCLUDED.ip_addr, port = EXCLUDED.port, location = EXCLUDED.location,
					last_received_from_sat_at = NOW()
				RETURNING xmax`,
				nodeID, ipAddr, port, loc)
			if err != nil {
				return merry.Wrap(err)
			}
			if xmax == "0" {
				newCount++
			}
			if loc != nil {
				locCount++
			}

			_, err = tx.Exec(`
				INSERT INTO storjnet.nodes_sat_offers (node_id,satellite_name,stamps) VALUES (?,?,array[now()])
				ON CONFLICT (node_id,satellite_name) DO UPDATE
				SET stamps = (
					SELECT array_agg(s)
					FROM unnest(nodes_sat_offers.stamps) AS s
					WHERE s > NOW() - INTERVAL '3 days'
				) || array[now()]`,
				nodeID, satelliteAddress)
			if err != nil {
				return merry.Wrap(err)
			}
		}
		return nil
	})
	log.Info().
		Int("total", len(limits)).Int("new", newCount).Int("with_location", locCount).
		TimeDiff("elapsed", time.Now(), stt).
		Msg("nodes saved")
	return merry.Wrap(err)
}

// config.dial
func dial(ctx context.Context, satelliteAddress string, apiKey *macaroon.APIKey) (_ *metaclient.Client, _ rpc.Dialer, fullNodeURL string, err error) {
	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  0,
		Concurrency: 1,
	})
	if err != nil {
		return nil, rpc.Dialer{}, "", merry.Wrap(err)
	}

	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}

	tlsOptions, err := tlsopts.NewOptions(ident, tlsConfig, nil)
	if err != nil {
		return nil, rpc.Dialer{}, "", merry.Wrap(err)
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)
	dialer.DialTimeout = 30 * time.Second
	// if config.DialContext != nil {
	// 	dialer.Transport = dialContextFunc(config.DialContext)
	// }

	nodeURL, err := storj.ParseNodeURL(satelliteAddress)
	if err != nil {
		return nil, rpc.Dialer{}, "", merry.Wrap(err)
	}

	// Node id is required in satelliteNodeID for all unknown (non-storj) satellites.
	// For known satellite it will be automatically prepended.
	if nodeURL.ID.IsZero() {
		nodeID, found := rpc.KnownNodeID(nodeURL.Address)
		if !found {
			return nil, rpc.Dialer{}, "", merry.New("node id is required in satelliteNodeURL")
		}
		satelliteAddress = storj.NodeURL{
			ID:      nodeID,
			Address: nodeURL.Address,
		}.String()
	}

	userAgent := ""
	metainfo, err := metaclient.DialNodeURL(ctx, dialer, satelliteAddress, apiKey, userAgent)

	return metainfo, dialer, satelliteAddress, merry.Wrap(err)
}
