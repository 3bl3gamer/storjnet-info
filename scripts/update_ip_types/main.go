package main

import (
	"storjnet/core"
	"storjnet/utils"
	"time"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog/log"
	"storj.io/common/storj"
)

func mainErr() error {
	db := utils.MakePGConnection()
	gdb, err := utils.MakeGeoIPCityConnection()
	if err != nil {
		return merry.Wrap(err)
	}
	asndb, err := utils.MakeGeoIPASNConnection()
	if err != nil {
		return merry.Wrap(err)
	}

	type Node struct {
		RawID  []byte
		IPAddr string
		IPType string
		Time   time.Time
	}

	fromTime := time.Time{}
	for {
		nodes := make([]Node, 100)
		_, err := db.Query(&nodes, `
			SELECT id AS raw_id, ip_addr, ip_type, last_received_from_sat_at AS time
			FROM nodes
			WHERE (ip_type IS NULL OR last_received_from_sat_at < NOW() - INTERVAL '7 days')
			  AND last_received_from_sat_at > ?
			  AND updated_at > NOW() - INTERVAL '1 day'
			ORDER BY last_received_from_sat_at ASC
			LIMIT 1000`,
			fromTime)
		if err != nil {
			return merry.Wrap(err)
		}
		if len(nodes) == 0 {
			break
		}

		log.Debug().Int("count", len(nodes)).Msg("nodes chunk")

		for _, node := range nodes {
			fromTime = node.Time

			nodeID, err := storj.NodeIDFromBytes(node.RawID)
			if err != nil {
				return merry.Wrap(err)
			}

			if rec := gdb.GetRecord(node.IPAddr); rec != nil {
				ipType, ok, err := core.FindCachedIPType(db, asndb, node.IPAddr)
				if err != nil {
					return merry.Wrap(err)
				}
				if ok && ipType != node.IPType {
					_, err := db.Exec(`UPDATE nodes SET ip_type = ? WHERE id = ?`, ipType, nodeID)
					if err != nil {
						return merry.Wrap(err)
					}
				}
			}
		}
	}
	return nil
}

func main() {
	if err := mainErr(); err != nil {
		panic(merry.Details(err))
	}
}
