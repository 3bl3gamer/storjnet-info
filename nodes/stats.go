package nodes

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"storjnet/core"
	"storjnet/utils"
	"sync"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v10"
	"storj.io/common/storj"
)

// satGroup is a set of satellites whose nodes get their own stats, see core.SatelliteGroup.
type satGroup struct {
	name     string
	satNames []string //as in nodes_sat_offers.satellite_name
}

// loadSatGroups groups all the satellites from which nodes have ever been received.
// A node belongs to a group if it was offered by any of the group satellites at least once:
// satellites do not offer full nodes, so "offered recently" would lose a lot of the alive ones.
func loadSatGroups(db *pg.DB) ([]satGroup, error) {
	var satNames []string
	_, err := db.Query(&satNames, `SELECT DISTINCT satellite_name FROM nodes_sat_offers`)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	namesByGroup := make(map[string][]string)
	for _, satName := range satNames {
		groupName := core.SatelliteGroup(satName)
		namesByGroup[groupName] = append(namesByGroup[groupName], satName)
	}
	groups := make([]satGroup, 0, len(namesByGroup))
	for groupName, names := range namesByGroup {
		sort.Strings(names)
		groups = append(groups, satGroup{name: groupName, satNames: names})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].name < groups[j].name })
	return groups, nil
}

// saveNodeStats saves one node_stats row per satellites group plus one for all the nodes,
// all with the same created_at.
func saveNodeStats(db *pg.DB, groups []satGroup, errors *[]error) {
	now := time.Now()
	*errors = append(*errors, saveGroupNodeStats(db, now, core.SatGroupAll, nil))
	for _, group := range groups {
		*errors = append(*errors, saveGroupNodeStats(db, now, group.name, group.satNames))
	}
}

// saveGroupNodeStats calculates stats over the nodes that have ever been offered by any of satNames.
// Empty satNames means no filtering (all nodes, all satellites).
func saveGroupNodeStats(db *pg.DB, now time.Time, groupName string, satNames []string) error {
	nodesFilter, offersFilter := "TRUE", "TRUE"
	if len(satNames) > 0 {
		nodesFilter = `EXISTS (SELECT 1 FROM nodes_sat_offers WHERE node_id = nodes.id AND satellite_name = ANY(?sat_names))`
		offersFilter = `satellite_name = ANY(?sat_names)`
	}
	// named params, go-pg takes them from the fields of the last argument
	params := &struct {
		CreatedAt time.Time
		SatGroup  string
		SatNames  []string `pg:",array"`
	}{now, groupName, satNames}
	_, err := db.Exec(`
	WITH sel_nodes AS (
		SELECT id, ip_addr, port, location, asn, updated_at, tcp_updated_at, quic_updated_at
		FROM nodes
		WHERE `+nodesFilter+`
	), sel_offers AS (
		SELECT node_id, satellite_name, stamps[array_upper(stamps, 1)] AS last_stamp
		FROM nodes_sat_offers
		WHERE `+offersFilter+`
	)
	INSERT INTO storjnet.node_stats (
		created_at,
		sat_group,
		count_total,
		active_count_hours,
		active_count_proto,
		all_sat_offers_count_hours,
		per_sat_offers_count_hours,
		countries,
		countries_isp,
		subnet_countries,
		subnet_countries_isp,
		subnets_count,
		subnets_top,
		subnet_sizes,
		ip_types,
		ip_types_asn_tops,
		ports
	) VALUES (?created_at, ?sat_group, (
		-- count_total
		SELECT count(*) FROM sel_nodes
	), (
		-- active_count_hours
		SELECT jsonb_object_agg(
			hours, (SELECT count(*) FROM sel_nodes WHERE updated_at > NOW() - hours::float * INTERVAL '1 hour')
			ORDER BY hours
		)
		FROM (SELECT generate_series(1, 24) AS hours UNION SELECT unnest(ARRAY[48, 72, 0.5])) t
	), (
		-- active_count_proto
		SELECT jsonb_build_object(
			'tcp', (SELECT count(*) FROM sel_nodes WHERE tcp_updated_at > NOW() - INTERVAL '1 day'),
			'quic', (SELECT count(*) FROM sel_nodes WHERE quic_updated_at > NOW() - INTERVAL '1 day')
		)
	), (
		-- all_sat_offers_count_hours
		SELECT jsonb_object_agg(
			hours, (
				SELECT count(DISTINCT node_id) FROM sel_offers
				WHERE last_stamp > NOW() - hours::int * INTERVAL '1 hour'
			)
			ORDER BY hours
		)
		FROM unnest(ARRAY[1, 3, 6, 12, 24, 48, 72, 168, 720]) AS hours
	), (
		-- per_sat_offers_count_hours
		SELECT COALESCE(jsonb_object_agg(
			cur_sat_name, (
				SELECT jsonb_object_agg(
					hours, (
						SELECT count(*) FROM sel_offers
						WHERE satellite_name = cur_sat_name
						  AND last_stamp > NOW() - hours::int * INTERVAL '1 hour'
					)
					ORDER BY hours
				)
				FROM unnest(ARRAY[1, 3, 6, 12, 24, 48, 72, 168, 720]) AS hours
			)
			ORDER BY cur_sat_name
		), '{}')
		FROM (
			SELECT DISTINCT satellite_name AS cur_sat_name
			FROM sel_offers
			WHERE last_stamp > NOW() - INTERVAL '1 day'
		) AS t
	), (
		-- countries
		SELECT COALESCE(jsonb_object_agg(country, cnt), '{}') FROM (
			SELECT COALESCE(location->>'country', '<unknown>') AS country, count(*) AS cnt
			FROM sel_nodes AS nodes
			WHERE updated_at > NOW() - INTERVAL '1 day'
			GROUP BY country
		) AS t
	), (
		-- countries_isp
		SELECT COALESCE(jsonb_object_agg(country, cnt), '{}') FROM (
			SELECT COALESCE(location->>'country', '<unknown>') AS country, count(*) AS cnt
			FROM sel_nodes AS nodes
			JOIN autonomous_systems ON nodes.asn = autonomous_systems.number
			WHERE nodes.updated_at > NOW() - INTERVAL '1 day'
			  AND COALESCE(NULLIF(ipinfo->>'type', ''), NULLIF(incolumitas->>'type', '')) = 'isp'
			GROUP BY country
		) AS t
	), (
		-- subnet_countries
		SELECT COALESCE(jsonb_object_agg(country, cnt), '{}') FROM (
			SELECT country, count(*) as cnt
			FROM (
				SELECT
					COALESCE(location->>'country', '<unknown>') AS country,
					host(set_masklen(ip_addr, 24)::cidr) AS net
				FROM sel_nodes AS nodes
				WHERE updated_at > NOW() - INTERVAL '1 day'
				GROUP BY country, net
			) AS t
			GROUP BY country
		) AS t
	), (
		-- subnet_countries_isp
		SELECT COALESCE(jsonb_object_agg(country, cnt), '{}') FROM (
			SELECT country, count(*) as cnt
			FROM (
				SELECT
					COALESCE(location->>'country', '<unknown>') AS country,
					host(set_masklen(ip_addr, 24)::cidr) AS net
				FROM sel_nodes AS nodes
				JOIN autonomous_systems ON nodes.asn = autonomous_systems.number
				WHERE updated_at > NOW() - INTERVAL '1 day'
				  AND COALESCE(NULLIF(ipinfo->>'type', ''), NULLIF(incolumitas->>'type', '')) = 'isp'
				GROUP BY country, net
			) AS t
			GROUP BY country
		) AS t
	), (
		-- subnets_count
		SELECT COUNT(DISTINCT host(set_masklen(ip_addr, 24)::cidr))
		FROM sel_nodes AS nodes
		WHERE updated_at > NOW() - INTERVAL '1 day'
	), (
		-- subnets_top
		SELECT COALESCE(jsonb_object_agg(net, size), '{}') FROM (
			SELECT net, count(*) as size FROM (
				SELECT host(set_masklen(ip_addr, 24)::cidr) AS net
				FROM sel_nodes AS nodes
				WHERE updated_at > NOW() - INTERVAL '1 day'
			) AS t
			GROUP BY net HAVING count(*) >= 10
		) AS t
	), (
		-- subnet_sizes
		SELECT COALESCE(jsonb_object_agg(size, cnt), '{}') FROM (
			SELECT size, count(*) as cnt FROM (
				SELECT net, count(*) as size FROM (
					SELECT host(set_masklen(ip_addr, 24)::cidr) AS net
					FROM sel_nodes AS nodes
					WHERE updated_at > NOW() - INTERVAL '1 day'
				) AS t
				GROUP BY net
			) AS t
			GROUP BY size
		) AS t
	), (
		-- ip_types
		SELECT COALESCE(jsonb_object_agg(ip_type, cnt), '{}') FROM (
			SELECT
				COALESCE((SELECT COALESCE(NULLIF(ipinfo->>'type', ''), NULLIF(incolumitas->>'type', ''))
					FROM autonomous_systems WHERE number = nodes.asn), '<unknown>') AS ip_type,
				count(*) AS cnt
			FROM sel_nodes AS nodes
			WHERE updated_at > NOW() - INTERVAL '1 day'
			GROUP BY ip_type
		) AS t
	), (
		-- ip_types_asn_tops
		SELECT COALESCE(jsonb_object_agg(ip_type, tops), '{}') FROM (
			SELECT ip_type, json_object_agg(COALESCE(asn::text, '<unknown>'), count) AS tops
			FROM (
				SELECT asn, ip_type, count(*), row_number() OVER (PARTITION BY ip_type ORDER BY count(*) DESC)
				FROM (
					SELECT asn,
						COALESCE((SELECT COALESCE(NULLIF(ipinfo->>'type', ''), NULLIF(incolumitas->>'type', ''), '<unknown>')
							FROM autonomous_systems WHERE number = nodes.asn), '<unknown>') AS ip_type
					FROM sel_nodes AS nodes
					WHERE updated_at > NOW() - INTERVAL '1 day'
				) AS t
				GROUP BY asn, ip_type
			) AS t
			WHERE row_number <= 10
			GROUP BY ip_type
		) AS t
	), (
		-- ports
		SELECT COALESCE(jsonb_object_agg(port, cnt), '{}') FROM (
			SELECT port, count(*) AS cnt
			FROM sel_nodes AS nodes
			WHERE updated_at > NOW() - INTERVAL '1 day'
			GROUP BY port
			LIMIT 100
		) AS t
	))`,
		params)
	return merry.Wrap(err)
}

// saveDailyStats saves which nodes are active today (and which have come and left since yesterday).
// The arrays of node_daily_stats hold node_nums.num instead of the 32-byte node IDs.
func saveDailyStats(db *pg.DB, groups []satGroup, errors *[]error) {
	// activeNodesSQL must return distinct node IDs as "id".
	// Nodes seen for the first time get their numbers right in this query (new_node_nums).
	// All parts of a query work with the same snapshot, so "SELECT FROM node_nums" in active_nodes
	// returns only the numbers that existed before the query, and the just inserted ones
	// can be obtained only through RETURNING. Hence the UNION ALL of both.
	dailyStats := func(kind, activeNodesSQL string, params ...interface{}) error {
		params = append(params, kind, kind, kind)
		_, err := db.Exec(`
			WITH dates (today, yesterday) AS (VALUES(
				(NOW() at time zone 'UTC')::date,
				(NOW() at time zone 'UTC')::date - INTERVAL '1 day'
			)), active_node_ids AS (
				`+activeNodesSQL+`
			), new_node_nums AS (
				INSERT INTO node_nums (node_id)
				SELECT a.id FROM active_node_ids AS a
				WHERE NOT EXISTS (SELECT 1 FROM node_nums WHERE node_id = a.id)
				RETURNING num
			), active_nodes AS (
				SELECT num FROM active_node_ids AS a JOIN node_nums ON node_id = a.id
				UNION ALL
				SELECT num FROM new_node_nums
			)
			INSERT INTO node_daily_stats (date, kind, node_ids, come_node_ids, left_node_ids)
			VALUES (
				(SELECT today FROM dates),
				?,
				(SELECT COALESCE(array_agg(num ORDER BY num), '{}'::integer[]) FROM active_nodes),
				(
					SELECT COALESCE(array_agg(num ORDER BY num), '{}'::integer[])
					FROM active_nodes
					LEFT OUTER JOIN (SELECT unnest(node_ids) AS yest_num FROM node_daily_stats, dates WHERE date = yesterday AND kind = ?) AS t
					ON (num = yest_num) WHERE yest_num IS NULL
				), (
					SELECT COALESCE(array_agg(yest_num ORDER BY yest_num), '{}'::integer[])
					FROM unnest((SELECT node_ids FROM node_daily_stats, dates WHERE date = yesterday AND kind = ?)) as yest_num
					WHERE NOT EXISTS (SELECT 1 FROM active_nodes WHERE num = yest_num)
				)
			)
			ON CONFLICT (date, kind) DO UPDATE SET
				node_ids = EXCLUDED.node_ids,
				come_node_ids = EXCLUDED.come_node_ids,
				left_node_ids = EXCLUDED.left_node_ids`,
			params...)
		return merry.Wrap(err)
	}

	err := dailyStats("active", `
		SELECT id FROM nodes WHERE updated_at > NOW() - INTERVAL '24 hours'`)
	*errors = append(*errors, merry.Wrap(err))

	for _, group := range groups {
		err = dailyStats("active:"+group.name, `
			SELECT id FROM nodes
			WHERE updated_at > NOW() - INTERVAL '24 hours'
			  AND EXISTS (SELECT 1 FROM nodes_sat_offers WHERE node_id = nodes.id AND satellite_name IN (?))`,
			pg.In(group.satNames))
		*errors = append(*errors, merry.Wrap(err))
	}

	err = dailyStats("offered_by_sats", `
		SELECT DISTINCT node_id AS id FROM nodes_sat_offers
		WHERE stamps[array_upper(stamps, 1)] > NOW() - INTERVAL '24 hours'`)
	*errors = append(*errors, merry.Wrap(err))

	var satNames []string
	_, err = db.Query(&satNames, `
		SELECT DISTINCT satellite_name FROM nodes_sat_offers
		WHERE stamps[array_upper(stamps, 1)] > NOW() - INTERVAL '24 hours'`)
	*errors = append(*errors, merry.Wrap(err))

	for _, satName := range satNames {
		err = dailyStats("offered_by_sat:"+satName, `
			SELECT node_id AS id FROM nodes_sat_offers
			WHERE satellite_name = ?
			  AND stamps[array_upper(stamps, 1)] > NOW() - INTERVAL '24 hours'`,
			satName)
		*errors = append(*errors, merry.Wrap(err))
	}
}

type OffWithSatDetails interface {
	SetSatDetails(id storj.NodeID, host string)
}
type OffSatelliteDetails struct {
	SatelliteID   storj.NodeID `json:"-"`
	SatelliteHost string       `json:"-"`
}

func (d *OffSatelliteDetails) SetSatDetails(id storj.NodeID, host string) {
	d.SatelliteID = id
	d.SatelliteHost = host
}

type OffDataStat struct {
	OffSatelliteDetails
	BandwidthBytesDownloaded         int64   `pg:",use_zero" json:"bandwidth_bytes_downloaded"`           //number of bytes downloaded (egress) from the network for the last 30 days
	BandwidthBytesUploaded           int64   `pg:",use_zero" json:"bandwidth_bytes_uploaded"`             //number of bytes uploaded (ingress) to the network for the last 30 days
	StorageInlineBytes               int64   `pg:",use_zero" json:"storage_inline_bytes"`                 //number of bytes stored in inline segments on the satellite
	StorageInlineSegments            int64   `pg:",use_zero" json:"storage_inline_segments"`              //number of segments stored inline on the satellite
	StorageMedianHealthyPiecesCount  float64 `pg:",use_zero" json:"storage_median_healthy_pieces_count"`  //median number of healthy pieces per segment stored on storage nodes
	StorageMinHealthyPiecesCount     int64   `pg:",use_zero" json:"storage_min_healthy_pieces_count"`     //inimum number of healthy pieces per segment stored on storage nodes
	StorageRemoteBytes               int64   `pg:",use_zero" json:"storage_remote_bytes"`                 //number of bytes stored on storage nodes (does not take into account the expansion factor of erasure encoding)
	StorageRemoteSegments            int64   `pg:",use_zero" json:"storage_remote_segments"`              //number of segments stored on storage nodes
	StorageRemoteSegments_lost       int64   `pg:",use_zero" json:"storage_remote_segments_lost"`         //number of irreparable segments lost from storage nodes
	StorageTotalBytes                int64   `pg:",use_zero" json:"storage_total_bytes"`                  //total number of bytes (both inline and remote) stored on the network
	StorageTotalObjects              int64   `pg:",use_zero" json:"storage_total_objects"`                //total number of objects stored on the network
	StorageTotalPieces               int64   `pg:",use_zero" json:"storage_total_pieces"`                 //total number of pieces stored on storage nodes
	StorageTotalSegments             int64   `pg:",use_zero" json:"storage_total_segments"`               //total number of segments stored on storage nodes
	StorageFreeCapacityEstimateBytes int64   `pg:",use_zero" json:"storage_free_capacity_estimate_bytes"` //statistical estimate of free storage node capacity, with suspicious values removed
	StorageTotalBytesAfterExpansion  float64 `pg:",use_zero" json:"storage_total_bytes_after_expansion"`  //number of bytes stored on storage nodes with erasure encoding expansion (official stats have it fractional)
}

// OffDataPublicStat is data-public.json: like OffDataStat but only for the public placement and with fewer fields.
type OffDataPublicStat struct {
	OffSatelliteDetails
	BandwidthBytesDownloaded int64 `pg:",use_zero" json:"bandwidth_bytes_downloaded"`
	BandwidthBytesUploaded   int64 `pg:",use_zero" json:"bandwidth_bytes_uploaded"`
	StorageTotalBytes        int64 `pg:",use_zero" json:"storage_total_bytes"`
	StorageTotalObjects      int64 `pg:",use_zero" json:"storage_total_objects"`
	StorageTotalSegments     int64 `pg:",use_zero" json:"storage_total_segments"`
}

type OffNodeStat struct {
	OffSatelliteDetails
	ActiveNodes       int64 `pg:",use_zero" json:"active_nodes"`       //number of storage nodes that were successfully contacted within the last 4 hours, excludes disqualified and exited nodes
	DisqualifiedNodes int64 `pg:",use_zero" json:"disqualified_nodes"` //number of disqualified storage nodes
	ExitedNodes       int64 `pg:",use_zero" json:"exited_nodes"`       //number of storage nodes that gracefully exited the satellite, excludes disqualified nodes
	OfflineNodes      int64 `pg:",use_zero" json:"offline_nodes"`      //number of storage nodes that were not successfully contacted within the last 4 hours, excludes disqualified and exited nodes
	SuspendedNodes    int64 `pg:",use_zero" json:"suspended_nodes"`    //number of suspended storage nodes, excludes disqualified and exited nodes
	TotalNodes        int64 `pg:",use_zero" json:"total_nodes"`        //total number of unique storage nodes that ever contacted the satellite
	VettedNodes       int64 `pg:",use_zero" json:"vetted_nodes"`       //number of vetted storage nodes, excludes disqualified and exited nodes
	FullNodes         int64 `pg:",use_zero" json:"full_nodes"`         //number of storage nodes without free disk
	ActiveWallets     int64 `pg:",use_zero" json:"active_wallets"`     //number of distinct wallets of active nodes
	// Only official stats have it. It is a pointer for the missing field to be saved as NULL, not as zero.
	ActiveSelectNodes *int64 `json:"active_select_nodes"` //number of active nodes of the Storj Select tier
}

type OffAccountStat struct {
	OffSatelliteDetails
	RegisteredAccounts int64 `pg:",use_zero" json:"registered_accounts"` //number of registered user accounts
}

func getJSON(url string, obj interface{}) error {
	maxRetries := 3

	transport := &http.Transport{
		// Forcing HTTP/1 (an empty, non-nil map disables HTTP/2 automatic upgrades).
		// They are breaking the Internet. HTTP/2 responses get stuck after ~14K bytes received.
		// For some reason, HTTP/1 still works.
		// TODO: RKS
		TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	var err error
	var resp *http.Response
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Second)
		}
		resp, err = client.Get(url)
		retriable := true
		if err == nil && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			err = merry.Errorf("unexpected response status: %s", resp.Status)
			// the file is just not there (or not for us), asking again will not help
			retriable = resp.StatusCode >= 500
		}
		if err == nil || !retriable {
			break
		}
		fmt.Fprintf(os.Stderr, "WARN: failed to fetch %s: %s, attempt %d of %d\n", url, err, i+1, maxRetries)
	}
	if err != nil {
		return merry.Wrap(err)
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(obj)
	return merry.Wrap(err)
}

func saveSatDetails(db *pg.DB, satName string, satData OffWithSatDetails) error {
	satID, satHost, err := core.SatelliteIDAndHost(satName)
	if err != nil {
		return merry.Wrap(err)
	}
	satData.SetSatDetails(satID, satHost)
	if _, err := db.Model(satData).Insert(); err != nil {
		return merry.Wrap(err)
	}
	return nil
}

// Public stats from the satellites.
var offStatsSources = []struct {
	baseURL     string
	hasAccounts bool //accounts.json
}{
	{baseURL: "https://stats.storjshare.io", hasAccounts: true},
	{baseURL: "https://stats.storkspace.net", hasAccounts: false},
}

// saveOffStatsFile fetches {"id@host:port": {...stats...}, ...} and saves one row per satellite.
func saveOffStatsFile[T any, PT interface {
	*T
	OffWithSatDetails
}](db *pg.DB, url string, errChan chan<- error) {
	data := make(map[string]T)
	if err := getJSON(url, &data); err != nil {
		errChan <- merry.Prepend(err, url)
		return
	}
	for satName, satData := range data {
		if err := saveSatDetails(db, satName, PT(&satData)); err != nil {
			errChan <- merry.Prepend(err, url)
		}
	}
}

func saveOffStats(db *pg.DB, errors *[]error) {
	wg := sync.WaitGroup{}
	errChan := make(chan error, 8)
	start := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f()
		}()
	}

	// sources are independent: a failed one must not prevent saving the others
	for _, source := range offStatsSources {
		baseURL := source.baseURL
		start(func() {
			saveOffStatsFile[OffDataStat](db, baseURL+"/data.json", errChan)
		})
		start(func() {
			saveOffStatsFile[OffDataPublicStat](db, baseURL+"/data-public.json", errChan)
		})
		start(func() {
			saveOffStatsFile[OffNodeStat](db, baseURL+"/nodes.json", errChan)
		})
		if source.hasAccounts {
			start(func() {
				saveOffStatsFile[OffAccountStat](db, baseURL+"/accounts.json", errChan)
			})
		}
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		*errors = append(*errors, merry.Wrap(err))
	}
}

func SaveStats(nodeStats, dailyStats, offStats bool) error {
	db := utils.MakePGConnection()

	var errors []error
	if nodeStats || dailyStats {
		groups, err := loadSatGroups(db)
		if err != nil {
			return merry.Wrap(err)
		}
		if nodeStats {
			saveNodeStats(db, groups, &errors)
		}
		if dailyStats {
			saveDailyStats(db, groups, &errors)
		}
	}
	if offStats {
		saveOffStats(db, &errors)
	}

	for _, err := range errors {
		if err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}
