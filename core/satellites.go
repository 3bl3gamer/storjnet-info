package core

import (
	"slices"
	"strings"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v10"
	"storj.io/common/storj"
)

const (
	// SatGroupAll is not a real group: it marks stats calculated over all the nodes regardless of satellites.
	SatGroupAll      = "all"
	SatGroupOfficial = "official"
)

// Satellite host -> stats group.
// A host starting with "*" is a suffix pattern.
var satHostGroups = map[string]string{
	"*.storj.io":                 SatGroupOfficial,
	"*.tardigrade.io":            SatGroupOfficial,
	"satellite.stefan-benten.de": SatGroupOfficial,
	"sj.storkspace.net":          "storkspace",
}

// Test satellites, to be skipped (from stats.storkspace.net/data.json, etc.).
var testSatHosts = map[string]struct{}{
	"tutor.storkspace.net": {},
}

// IsTestSatellite tells if the satellite ("id@host:port" or just host) belongs to a test network.
// Stats of such satellites are not saved at all.
func IsTestSatellite(nameOrHost string) bool {
	_, ok := testSatHosts[strings.ToLower(satelliteHost(nameOrHost))]
	return ok
}

// SatGroups returns the known stats groups: "official", then the third-party ones by name.
func SatGroups() []string {
	groups := []string{SatGroupOfficial}
	for _, group := range satHostGroups {
		if !slices.Contains(groups, group) {
			groups = append(groups, group)
		}
	}
	slices.Sort(groups[1:])
	return groups
}

// SatUsernames returns the users whose nodes are the satellites pinged for the main page:
// one user per group, named "satellites.<group>".
func SatUsernames() []string {
	names := SatGroups()
	for i, group := range names {
		names[i] = "satellites." + group
	}
	return names
}

// LoadSatNamesByGroupSlow groups all the satellites from which nodes have ever been received:
// group -> sorted names as in nodes_sat_offers.satellite_name.
// Slow! Use with caution.
func LoadSatNamesByGroupSlow(db *pg.DB) (map[string][]string, error) {
	var satNames []string
	_, err := db.Query(&satNames, `SELECT DISTINCT satellite_name FROM nodes_sat_offers ORDER BY satellite_name`)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	namesByGroup := make(map[string][]string)
	for _, satName := range satNames {
		groupName := SatelliteGroup(satName)
		namesByGroup[groupName] = append(namesByGroup[groupName], satName)
	}
	return namesByGroup, nil
}

// SatelliteIDAndHost splits "id@host:port" (port is optional).
// The id is optional too: for a plain "host:port" it is zero (stats.storkspace.net has no satellite IDs).
func SatelliteIDAndHost(name string) (storj.NodeID, string, error) {
	var id storj.NodeID
	if idStr, _, hasID := strings.Cut(name, "@"); hasID {
		var err error
		if id, err = storj.NodeIDFromString(idStr); err != nil {
			return storj.NodeID{}, "", merry.Wrap(err)
		}
	}
	host := satelliteHost(name)
	if host == "" {
		return storj.NodeID{}, "", merry.Errorf("no host in satellite name: %q", name)
	}
	return id, host, nil
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
	if group, ok := satHostGroups[host]; ok {
		return group
	}
	// of several matching patterns the most specific (longest) one wins, map order must not matter
	group, matchLen := host, 0
	for pattern, patternGroup := range satHostGroups {
		suffix, isPattern := strings.CutPrefix(pattern, "*")
		if isPattern && len(suffix) > matchLen && strings.HasSuffix(host, suffix) {
			group, matchLen = patternGroup, len(suffix)
		}
	}
	return group
}
