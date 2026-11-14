package core

import (
	"strings"

	"github.com/ansel1/merry"
	"storj.io/common/storj"
)

const (
	// SatGroupAll is not a real group: it marks stats calculated over all the nodes regardless of satellites.
	SatGroupAll      = "all"
	SatGroupOfficial = "official"
)

var officialSatHostSuffixes = []string{".storj.io", ".tardigrade.io"}

var officialSatHosts = map[string]struct{}{
	"satellite.stefan-benten.de": {},
}

// Third-party satellites, host -> group name. Satellites of one operator may share a group.
var unofficialSatHostGroups = map[string]string{
	"sj.storkspace.net": "storkspace",
}

// SatelliteIDAndHost splits "id@host:port" (port is optional).
func SatelliteIDAndHost(name string) (storj.NodeID, string, error) {
	hostIndex := strings.Index(name, "@")
	if hostIndex < 0 {
		return storj.NodeID{}, "", merry.Errorf("'@' not found in node name: %s", name)
	}
	id, err := storj.NodeIDFromString(name[:hostIndex])
	if err != nil {
		return storj.NodeID{}, "", merry.Wrap(err)
	}
	return id, satelliteHost(name), nil
}

func satelliteHost(nameOrHost string) string {
	host := nameOrHost[strings.Index(nameOrHost, "@")+1:]
	if portIndex := strings.LastIndex(host, ":"); portIndex >= 0 {
		host = host[:portIndex]
	}
	return host
}

// SatelliteGroup returns the stats group of a satellite by its "id@host:port" name or just by its host.
// An unknown satellite makes a group of its own (named by the host) instead of being silently counted as official.
func SatelliteGroup(nameOrHost string) string {
	host := strings.ToLower(satelliteHost(nameOrHost))
	if group, ok := unofficialSatHostGroups[host]; ok {
		return group
	}
	if _, ok := officialSatHosts[host]; ok {
		return SatGroupOfficial
	}
	for _, suffix := range officialSatHostSuffixes {
		if strings.HasSuffix(host, suffix) {
			return SatGroupOfficial
		}
	}
	return host
}
