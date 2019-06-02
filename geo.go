package main

import (
	"github.com/abh/geoip"
	"storj.io/storj/pkg/pb"
)

func StartLocationSearcher(gdb *geoip.GeoIP, kadDataRawChan chan *pb.Node, kadDataForSaveChan chan *KadDataExt) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		for nodeRaw := range kadDataRawChan {
			node := &KadDataExt{Node: nodeRaw, Location: nil}
			if nodeRaw.LastIp != "" {
				if rec := gdb.GetRecord(nodeRaw.LastIp); rec != nil {
					// fmt.Printf("%#v\n", rec)
					node.Location = &NodeLocation{
						Country:   rec.CountryName,
						City:      rec.City,
						Longitude: rec.Longitude,
						Latitude:  rec.Latitude,
					}
				}
			}
			kadDataForSaveChan <- node
		}
	}()
	return worker
}
