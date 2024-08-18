package nodes

import (
	"encoding/json"
	"net/http"
	"storjnet/utils"
	"strings"
	"sync"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"storj.io/common/storj"
)

func saveNodeStats(db *pg.DB, errors *[]error) {
	_, err := db.Exec(`
	INSERT INTO storjnet.node_stats (
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
		ports
	) VALUES ((
		-- count_total
		SELECT count(*) FROM nodes
	), (
		-- active_count_hours
		SELECT jsonb_object_agg(
			hours, (SELECT count(*) FROM nodes WHERE updated_at > NOW() - hours::float * INTERVAL '1 hour')
			ORDER BY hours
		)
		FROM (SELECT generate_series(1, 24) AS hours UNION SELECT unnest(ARRAY[48, 72, 0.5])) t
	), (
		-- active_count_proto
		SELECT jsonb_build_object(
			'tcp', (SELECT count(*) FROM nodes WHERE tcp_updated_at > NOW() - INTERVAL '1 day'),
			'quic', (SELECT count(*) FROM nodes WHERE quic_updated_at > NOW() - INTERVAL '1 day')
		)
	), (
		-- all_sat_offers_count_hours
		SELECT jsonb_object_agg(
			hours, (
				SELECT count(DISTINCT node_id) FROM nodes_sat_offers
				WHERE stamps[array_upper(stamps, 1)] > NOW() - hours::int * INTERVAL '1 hour'
			)
			ORDER BY hours
		)
		FROM unnest(ARRAY[1, 3, 6, 12, 24, 48, 72]) AS hours
	), (
		-- per_sat_offers_count_hours
		SELECT jsonb_object_agg(
			cur_sat_name, (
				SELECT jsonb_object_agg(
					hours, (
						SELECT count(*) FROM nodes_sat_offers
						WHERE satellite_name = cur_sat_name
						  AND stamps[array_upper(stamps, 1)] > NOW() - hours::int * INTERVAL '1 hour'
					)
					ORDER BY hours
				)
				FROM unnest(ARRAY[1, 3, 6, 12, 24, 48, 72]) AS hours
			)
			ORDER BY cur_sat_name
		)
		FROM (
			SELECT DISTINCT satellite_name AS cur_sat_name
			FROM nodes_sat_offers
			WHERE stamps[array_upper(stamps, 1)] > NOW() - INTERVAL '1 day'
		) AS t
	), (
		-- countries
		SELECT jsonb_object_agg(country, cnt) FROM (
			SELECT COALESCE(location->>'country', '<unknown>') AS country, count(*) AS cnt
			FROM nodes
			WHERE updated_at > NOW() - INTERVAL '1 day'
			GROUP BY country
		) AS t
	), (
		-- countries_isp
		SELECT jsonb_object_agg(country, cnt) FROM (
			SELECT COALESCE(location->>'country', '<unknown>') AS country, count(*) AS cnt
			FROM nodes
			JOIN autonomous_systems ON nodes.asn = autonomous_systems.number
			WHERE nodes.updated_at > NOW() - INTERVAL '1 day'
			  AND COALESCE(NULLIF(ipinfo->>'type', ''), NULLIF(incolumitas->>'type', '')) = 'isp'
			GROUP BY country
		) AS t
	), (
		-- subnet_countries
		SELECT jsonb_object_agg(country, cnt) FROM (
			SELECT country, count(*) as cnt
			FROM (
				SELECT
					COALESCE(location->>'country', '<unknown>') AS country,
					host(set_masklen(ip_addr, 24)::cidr) AS net
				FROM nodes
				WHERE updated_at > NOW() - INTERVAL '1 day'
				GROUP BY country, net
			) AS t
			GROUP BY country
		) AS t
	), (
		-- subnet_countries_isp
		SELECT jsonb_object_agg(country, cnt) FROM (
			SELECT country, count(*) as cnt
			FROM (
				SELECT
					COALESCE(location->>'country', '<unknown>') AS country,
					host(set_masklen(ip_addr, 24)::cidr) AS net
				FROM nodes
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
		FROM nodes
		WHERE updated_at > NOW() - INTERVAL '1 day'
	), (
		-- subnets_top
		SELECT jsonb_object_agg(net, size) FROM (
			SELECT net, count(*) as size FROM (
				SELECT host(set_masklen(ip_addr, 24)::cidr) AS net
				FROM nodes
				WHERE updated_at > NOW() - INTERVAL '1 day'
			) AS t
			GROUP BY net HAVING count(*) >= 10
		) AS t
	), (
		-- subnet_sizes
		SELECT jsonb_object_agg(size, cnt) FROM (
			SELECT size, count(*) as cnt FROM (
				SELECT net, count(*) as size FROM (
					SELECT host(set_masklen(ip_addr, 24)::cidr) AS net
					FROM nodes
					WHERE updated_at > NOW() - INTERVAL '1 day'
				) AS t
				GROUP BY net
			) AS t
			GROUP BY size
		) AS t
	), (
		-- ip_types
		SELECT jsonb_object_agg(ip_type, cnt) FROM (
			SELECT
				COALESCE((SELECT COALESCE(NULLIF(ipinfo->>'type', ''), NULLIF(incolumitas->>'type', ''))
					FROM autonomous_systems WHERE number = nodes.asn), '<unknown>') AS ip_type,
				count(*) AS cnt
			FROM nodes
			WHERE updated_at > NOW() - INTERVAL '1 day'
			GROUP BY ip_type
		) AS t
	), (
		-- ports
		SELECT jsonb_object_agg(port, cnt) FROM (
			SELECT port, count(*) AS cnt
			FROM nodes
			WHERE updated_at > NOW() - INTERVAL '1 day'
			GROUP BY port
			LIMIT 100
		) AS t
	))`)
	*errors = append(*errors, merry.Wrap(err))
}

func saveDailyStats(db *pg.DB, errors *[]error) {
	dailyStats := func(kind, activeNodesSQL string, params ...interface{}) error {
		params = append(params, kind, kind, kind)
		_, err := db.Exec(`
			WITH dates (today, yesterday) AS (VALUES(
				(NOW() at time zone 'UTC')::date,
				(NOW() at time zone 'UTC')::date - INTERVAL '1 day'
			)), active_nodes AS (
				`+activeNodesSQL+`
			)
			INSERT INTO node_daily_stats (date, kind, node_ids, come_node_ids, left_node_ids)
			VALUES (
				(SELECT today FROM dates),
				?,
				(SELECT array_agg(id ORDER BY id) FROM active_nodes),
				(
					SELECT COALESCE(array_agg(id ORDER BY id), '{}'::bytea[])
					FROM active_nodes
					LEFT OUTER JOIN (SELECT unnest(node_ids) AS yest_id FROM node_daily_stats, dates WHERE date = yesterday AND kind = ?) AS t
					ON (id = yest_id) WHERE yest_id IS NULL
				), (
					SELECT COALESCE(array_agg(yest_id ORDER BY yest_id), '{}'::bytea[])
					FROM unnest((SELECT node_ids FROM node_daily_stats, dates WHERE date = yesterday AND kind = ?)) as yest_id
					WHERE NOT EXISTS (SELECT 1 FROM active_nodes WHERE id = yest_id)
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
	BandwidthBytesDownloaded         int64 `pg:",use_zero" json:"bandwidth_bytes_downloaded"`           //number of bytes downloaded (egress) from the network for the last 30 days
	BandwidthBytesUploaded           int64 `pg:",use_zero" json:"bandwidth_bytes_uploaded"`             //number of bytes uploaded (ingress) to the network for the last 30 days
	StorageInlineBytes               int64 `pg:",use_zero" json:"storage_inline_bytes"`                 //number of bytes stored in inline segments on the satellite
	StorageInlineSegments            int64 `pg:",use_zero" json:"storage_inline_segments"`              //number of segments stored inline on the satellite
	StorageMedianHealthyPiecesCount  int64 `pg:",use_zero" json:"storage_median_healthy_pieces_count"`  //median number of healthy pieces per segment stored on storage nodes
	StorageMinHealthyPiecesCount     int64 `pg:",use_zero" json:"storage_min_healthy_pieces_count"`     //inimum number of healthy pieces per segment stored on storage nodes
	StorageRemoteBytes               int64 `pg:",use_zero" json:"storage_remote_bytes"`                 //number of bytes stored on storage nodes (does not take into account the expansion factor of erasure encoding)
	StorageRemoteSegments            int64 `pg:",use_zero" json:"storage_remote_segments"`              //number of segments stored on storage nodes
	StorageRemoteSegments_lost       int64 `pg:",use_zero" json:"storage_remote_segments_lost"`         //number of irreparable segments lost from storage nodes
	StorageTotalBytes                int64 `pg:",use_zero" json:"storage_total_bytes"`                  //total number of bytes (both inline and remote) stored on the network
	StorageTotalObjects              int64 `pg:",use_zero" json:"storage_total_objects"`                //total number of objects stored on the network
	StorageTotalPieces               int64 `pg:",use_zero" json:"storage_total_pieces"`                 //total number of pieces stored on storage nodes
	StorageTotalSegments             int64 `pg:",use_zero" json:"storage_total_segments"`               //total number of segments stored on storage nodes
	StorageFreeCapacityEstimateBytes int64 `pg:",use_zero" json:"storage_free_capacity_estimate_bytes"` //statistical estimate of free storage node capacity, with suspicious values removed
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
}
type OffAccountStat struct {
	OffSatelliteDetails
	RegisteredAccounts int64 `pg:",use_zero" json:"registered_accounts"` //number of registered user accounts
}

func getJSON(url string, obj interface{}) error {
	maxRetries := 3

	var err error
	var resp *http.Response
	for i := 0; i < maxRetries; i++ {
		resp, err = http.Get(url)
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		return merry.Wrap(err)
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(obj)
	return merry.Wrap(err)
}
func satIDAndHostFromName(name string) (storj.NodeID, string, error) {
	hostIndex := strings.Index(name, "@")
	if hostIndex < 0 {
		return storj.NodeID{}, "", merry.Errorf("'@' not found in node name: %s", name)
	}
	id, err := storj.NodeIDFromString(name[:hostIndex])
	if err != nil {
		return storj.NodeID{}, "", merry.Wrap(err)
	}
	portIndex := strings.LastIndex(name, ":")
	if portIndex < 0 {
		portIndex = len(name)
	}
	return id, name[hostIndex+1 : portIndex], nil
}
func saveSatDetails(db *pg.DB, satName string, satData OffWithSatDetails) error {
	satID, satHost, err := satIDAndHostFromName(satName)
	if err != nil {
		return merry.Wrap(err)
	}
	satData.SetSatDetails(satID, satHost)
	if _, err := db.Model(satData).Insert(); err != nil {
		return merry.Wrap(err)
	}
	return nil
}
func saveOffStats(db *pg.DB, errors *[]error) {
	wg := sync.WaitGroup{}
	errChan := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		data := make(map[string]OffDataStat)
		if err := getJSON("https://stats.storjshare.io/data.json", &data); err != nil {
			errChan <- err
			return
		}
		for satName, satData := range data {
			if err := saveSatDetails(db, satName, &satData); err != nil {
				errChan <- err
				continue
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		data := make(map[string]OffNodeStat)
		if err := getJSON("https://stats.storjshare.io/nodes.json", &data); err != nil {
			errChan <- err
			return
		}
		for satName, satData := range data {
			if err := saveSatDetails(db, satName, &satData); err != nil {
				errChan <- err
				continue
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		data := make(map[string]OffAccountStat)
		if err := getJSON("https://stats.storjshare.io/accounts.json", &data); err != nil {
			errChan <- err
			return
		}
		for satName, satData := range data {
			if err := saveSatDetails(db, satName, &satData); err != nil {
				errChan <- err
				continue
			}
		}
	}()

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
	if nodeStats {
		saveNodeStats(db, &errors)
	}
	if dailyStats {
		saveDailyStats(db, &errors)
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
