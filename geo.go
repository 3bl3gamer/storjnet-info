package main

import (
	"net"
	"strings"

	"github.com/abh/geoip"
	"storj.io/storj/pkg/pb"
)

func splitToHostAndPort(addr string) (string, string) {
	index := strings.LastIndexByte(addr, ':')
	if index == -1 {
		return "", ""
	}
	return addr[:index], addr[index+1:]
}

func StartLocationSearcher(gdb *geoip.GeoIP, kadDataRawChan chan *pb.Node, kadDataForSaveChan chan *KadDataExt) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		for nodeRaw := range kadDataRawChan {
			node := &KadDataExt{Node: nodeRaw, Location: nil}
			ip := nodeRaw.LastIp
			if ip == "" {
				host, _ := splitToHostAndPort(nodeRaw.Address.Address)
				ips, err := net.LookupHost(host)
				if err == nil {
					ip = ips[0]
				} else {
					logWarn("GEO-LOOKUP", "addr '%s' lookup error: %s", nodeRaw.Address.Address, err)
				}
			}
			if ip != "" {
				if rec := gdb.GetRecord(ip); rec != nil {
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
