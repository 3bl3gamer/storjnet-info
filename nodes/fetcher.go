package nodes

import (
	"context"
	"storjnet/core"
	"storjnet/utils"
	"strconv"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v10"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/uplink/private/metaclient"
)

func FetchAndProcess(satelliteAddress string, socksProxy string) error {
	apiKey, err := utils.RequireEnv("STORJ_API_KEY")
	if err != nil {
		return merry.Wrap(err)
	}

	db := utils.MakePGConnection()
	gdb, err := utils.OpenGeoIPConn("GeoLite2-City.mmdb")
	if err != nil {
		return merry.Wrap(err)
	}
	asndb, err := utils.OpenGeoIPConn("GeoLite2-ASN.mmdb")
	if err != nil {
		return merry.Wrap(err)
	}
	ctx := context.Background()

	var proxyDialer proxy.ContextDialer
	if socksProxy != "" {
		proxyDialer, err = parseSocksProxy(socksProxy)
		if err != nil {
			return merry.Wrap(err)
		}
	}

	parsedAPIKey, err := macaroon.ParseAPIKey(apiKey)
	if err != nil {
		return merry.Wrap(err)
	}
	metainfoClient, _, _, err := dial(ctx, satelliteAddress, parsedAPIKey, proxyDialer)
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
		Position: metaclient.SegmentPosition{
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
	return merry.Wrap(saveLimits(db, gdb, asndb, satelliteAddress, segResponse.Limits))
}

type NodeLocation struct {
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Longitude float32 `json:"longitude"`
	Latitude  float32 `json:"latitude"`
	Accuracy  int32   `json:"accuracy"`
}

func saveLimits(db *pg.DB, gdb, asndb *utils.GeoIPConn, satelliteAddress string, limits []*pb.AddressedOrderLimit) error {
	stt := time.Now()

	var asnsToUpdate []int64
	var ipsToUpdate []string

	newCount := 0
	locCount := 0
	ipTypeCount := 0
	err := db.RunInTransaction(context.Background(), func(tx *pg.Tx) error {
		ids := make([]storj.NodeID, len(limits))
		for i, l := range limits {
			ids[i] = l.Limit.StorageNodeId
		}
		_, err := tx.Exec(`SELECT 1 FROM nodes WHERE id IN (?) FOR UPDATE`, pg.In(ids))
		if err != nil {
			return merry.Wrap(err)
		}
		_, err = tx.Exec(`SELECT 1 FROM nodes_sat_offers WHERE node_id IN (?) FOR UPDATE`, pg.In(ids))
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
			var asn *int64
			{
				city, cityFound, err := gdb.CityStr(ipAddr)
				if err != nil {
					return merry.Wrap(err)
				}
				if cityFound {
					loc = &NodeLocation{
						Country:   utils.CountryA2ToA3IfExists(city.Country.IsoCode),
						City:      city.City.Names["en"],
						Longitude: float32(city.Location.Longitude),
						Latitude:  float32(city.Location.Latitude),
						Accuracy:  int32(city.Location.AccuracyRadius),
					}
					if city.Location.AccuracyRadius >= 1000 {
						_, err := db.QueryOne(&loc, `
							SELECT location FROM geoip_overrides
							WHERE network >>= ?::inet AND (location->'accuracy')::int < ?
							ORDER BY masklen(network) DESC
							LIMIT 1`,
							ipAddr, loc.Accuracy)
						if err != nil && err != pg.ErrNoRows {
							return merry.Wrap(err)
						}
					}
				}
			}
			{
				as, asFound, err := asndb.ASNStr(ipAddr)
				if err != nil {
					return merry.Wrap(err)
				}
				if asFound {
					asn_ := int64(as.AutonomousSystemNumber)
					asn = &asn_
					asnsToUpdate = append(asnsToUpdate, asn_)
				}
			}
			ipsToUpdate = append(ipsToUpdate, ipAddr)

			var xmax string
			_, err = tx.QueryOne(pg.Scan(&xmax), `
				INSERT INTO nodes
					(id, ip_addr, port, location, asn, last_received_from_sat_at) VALUES (?,?,?,?,?,NOW())
				ON CONFLICT (id) DO UPDATE SET
					ip_addr = EXCLUDED.ip_addr, port = EXCLUDED.port, location = EXCLUDED.location, asn = EXCLUDED.asn,
					last_received_from_sat_at = NOW()
				RETURNING xmax`,
				nodeID, ipAddr, port, loc, asn)
			if err != nil {
				return merry.Wrap(err)
			}
			if xmax == "0" {
				newCount++
			}
			if loc != nil {
				locCount++
			}
			if asn != nil {
				ipTypeCount++
			}

			_, err = tx.Exec(`
				INSERT INTO nodes_sat_offers (node_id,satellite_name,stamps) VALUES (?,?,array[now()])
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
		Int("total", len(limits)).Int("new", newCount).Int("with_location", locCount).Int("with_ip_type", ipTypeCount).
		TimeDiff("elapsed", time.Now(), stt).
		Msg("nodes saved")

	ipsUpdStart := time.Now()
	for _, ip := range ipsToUpdate {
		// if IPs updates took too long for some reason, skipping remaining updates,
		// otherwise a lot of fetcher processes can remain running and eat up all memory
		if time.Since(ipsUpdStart) > 10*time.Second {
			break
		}
		if _, err := core.UpdateIPCompanyIfNeed(db, ip); err != nil {
			log.Error().Err(err).Str("ip", ip).Msg("failed to update IP company")
			if merry.Is(err, core.ErrIncolumitasTooManyRequests) {
				break
			}
		}
	}

	asnUpdStart := time.Now()
	for _, asn := range asnsToUpdate {
		// if ASN updates took too long for some reason, skipping remaining updates,
		// otherwise a lot of fetcher processes can remain running and eat up all memory
		if time.Since(asnUpdStart) > 10*time.Second {
			break
		}
		if _, err := core.UpdateASInfoIfNeed(db, asn); err != nil {
			log.Error().Err(err).Int64("asn", asn).Msg("failed to update AS info")
			if merry.Is(err, core.ErrIncolumitasTooManyRequests) {
				break
			}
		}
	}
	return merry.Wrap(err)
}

// config.dial
func dial(ctx context.Context, satelliteAddress string, apiKey *macaroon.APIKey, proxyDialer proxy.ContextDialer) (_ *metaclient.Client, _ rpc.Dialer, fullNodeURL string, err error) {
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

	if proxyDialer != nil {
		dialer.Connector = rpc.NewDefaultTCPConnector(proxyDialer.DialContext)
	}

	nodeURL, err := storj.ParseNodeURL(satelliteAddress)
	if err != nil {
		return nil, rpc.Dialer{}, "", merry.Wrap(err)
	}

	// Node ID is required in satelliteNodeID for all unknown (non-storj) satellites.
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

func parseSocksProxy(cfg string) (proxy.ContextDialer, error) {
	items := strings.SplitN(cfg, ":", 4) //address:port or address:port:user:passwd
	if len(items) < 2 {
		return nil, merry.New("invalid socks proxy config: " + cfg)
	}

	address := items[0] + ":" + items[1]

	var auth *proxy.Auth
	if len(items) >= 3 {
		password := ""
		if len(items) >= 4 {
			password = items[3]
		}
		auth = &proxy.Auth{User: items[2], Password: password}
	}

	dialer, err := proxy.SOCKS5("tcp", address, auth, proxy.Direct)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	// SOCKS5() returns proxy.Dialer which in fact is also a proxy.ContextDialer
	ctxDialer := dialer.(proxy.ContextDialer)
	return ctxDialer, nil
}
