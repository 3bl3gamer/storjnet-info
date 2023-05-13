package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"storjnet/core"
	"storjnet/utils"
	"strconv"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"storj.io/common/storj"
)

var rootCmd = &cobra.Command{
	Use: os.Args[0],
}
var fillNodeASNsCmd = &cobra.Command{
	Use:  "fill-node-asns",
	RunE: CMDFillNodeASNs,
}
var fillASIPInfoDataCmd = &cobra.Command{
	Use:  "fill-as-ipinfo-data",
	RunE: CMDFillASIPInfoData,
}

func CMDFillNodeASNs(cmd *cobra.Command, args []string) error {
	db := utils.MakePGConnection()
	asndb, err := utils.MakeGeoIPASNConnection()
	if err != nil {
		return merry.Wrap(err)
	}

	type Node struct {
		RawID  []byte
		IPAddr string
		Asn    int64
		Time   time.Time
	}

	fromTime := time.Time{}
	for {
		nodes := make([]Node, 100)
		_, err := db.Query(&nodes, `
			SELECT id as raw_id, ip_addr, asn, last_received_from_sat_at AS time
			FROM nodes
			WHERE last_received_from_sat_at > ?
			  AND updated_at > NOW() - INTERVAL '4 days'
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

		err = db.RunInTransaction(func(tx *pg.Tx) error {
			for _, node := range nodes {
				fromTime = node.Time

				nodeID, err := storj.NodeIDFromBytes(node.RawID)
				if err != nil {
					return merry.Wrap(err)
				}

				geoipAsn, asnFound, err := core.FindIPAddrASN(asndb, node.IPAddr)
				if err != nil {
					return merry.Wrap(err)
				}
				if !asnFound {
					log.Warn().Str("IP", node.IPAddr).Msg("ASN not found")
					continue
				}

				if _, err := core.UpdateASInfo(db, geoipAsn); err != nil {
					log.Error().Err(err).Int64("asn", geoipAsn).Msg("failed to update AS info")
				}

				if node.Asn != geoipAsn {
					_, err = tx.Exec(`UPDATE nodes SET asn = ? WHERE id = ?`, geoipAsn, nodeID)
					if err != nil {
						return merry.Wrap(err)
					}
				}
			}
			return nil
		})
		if err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}

func CMDFillASIPInfoData(cmd *cobra.Command, args []string) error {
	ipInfoToken, err := utils.RequireEnv("IPINFO_TOKEN")
	if err != nil {
		return merry.Wrap(err)
	}

	db := utils.MakePGConnection()
	asndb, err := utils.MakeGeoIPASNConnection()
	if err != nil {
		return merry.Wrap(err)
	}

	type Node struct {
		IPAddr string
		Time   time.Time
	}

	savedASNs := make(map[int64]struct{})
	fromTime := time.Time{}
	for {
		nodes := make([]Node, 100)
		_, err := db.Query(&nodes, `
			SELECT ip_addr, last_received_from_sat_at AS time
			FROM nodes
			WHERE last_received_from_sat_at > ?
			  AND updated_at > NOW() - INTERVAL '3 days'
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

			geoipAsn, asnFound, err := core.FindIPAddrASN(asndb, node.IPAddr)
			if err != nil {
				return merry.Wrap(err)
			}
			if !asnFound {
				continue
			}

			if _, ok := savedASNs[geoipAsn]; !ok {
				var t int64
				_, err := db.Query(&t, `
					SELECT 1 FROM autonomous_systems
					WHERE number = ? AND ipinfo IS NOT NULL AND ipinfo_updated_at > NOW() - INTERVAL '1 day'`,
					geoipAsn)
				if t == 1 {
					continue
				}
				if err != nil {
					return merry.Wrap(err)
				}

				ipInfoStr, err := fetchIPInfo(node.IPAddr, ipInfoToken)
				if err != nil {
					return merry.Wrap(err)
				}
				type ASInfo struct {
					Asn    string
					Name   string
					Type   string
					Domain string
				}
				type IPInfo struct {
					Asn ASInfo
				}
				ipInfo := IPInfo{}
				if err := json.Unmarshal([]byte(ipInfoStr), &ipInfo); err != nil {
					return merry.Wrap(err)
				}
				ipInfoAsn, err := asnStr2int(ipInfo.Asn.Asn)
				if err != nil {
					return merry.Wrap(err)
				}

				asInfo := ipInfo.Asn

				if ipInfoAsn != geoipAsn {
					log.Warn().Int64("geoipAsn", geoipAsn).Int64("ipInfoAsn", ipInfoAsn).Msg("ASN mismatch")
					asInfoStr, err := fetchIPInfo("AS"+strconv.FormatInt(geoipAsn, 10), ipInfoToken)
					if err != nil {
						return merry.Wrap(err)
					}
					if err := json.Unmarshal([]byte(asInfoStr), &asInfo); err != nil {
						return merry.Wrap(err)
					}
				}

				savedAsn, err := asnStr2int(asInfo.Asn)
				if err != nil {
					return merry.Wrap(err)
				}
				_, err = db.Exec(`
					INSERT INTO autonomous_systems (number, ipinfo, ipinfo_updated_at)
					VALUES (?, jsonb_build_object('name', ?, 'type', ?, 'domain', ?), NOW())
					ON CONFLICT (number) DO UPDATE SET
						ipinfo = EXCLUDED.ipinfo,
						ipinfo_updated_at = EXCLUDED.ipinfo_updated_at`,
					savedAsn, ipInfo.Asn.Name, ipInfo.Asn.Type, ipInfo.Asn.Domain)
				if err != nil {
					return merry.Wrap(err)
				}

				savedASNs[savedAsn] = struct{}{}
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(fillNodeASNsCmd)
	rootCmd.AddCommand(fillASIPInfoDataCmd)
}

func main() {
	// Logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.ErrorStackMarshaler = func(err error) interface{} { return merry.Details(err) }
	zerolog.ErrorStackFieldName = "message" //TODO: https://github.com/rs/zerolog/issues/157
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05.000"})

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Msg(merry.Details(err))
	}
}

func fetchIPInfo(ip string, token string) (string, error) {
	req, err := http.NewRequest("GET", "https://ipinfo.io/"+ip+"?token="+token, nil)
	if err != nil {
		return "", merry.Wrap(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", merry.Wrap(err)
	}
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", merry.Wrap(err)
	}

	if resp.StatusCode != 200 {
		return "", merry.Errorf("ipinfo status %d: %s", resp.StatusCode, string(buf))
	}

	log.Debug().Str("IP", ip).Msg("fetched IP info")
	return string(buf), nil
}

func asnStr2int(asnStr string) (int64, error) {
	if !strings.HasPrefix(asnStr, "AS") {
		return 0, merry.Errorf("expected ASN to start with 'AS', got: %s", asnStr)
	}
	asn, err := strconv.ParseInt(asnStr[2:], 10, 64)
	if err != nil {
		return 0, merry.Wrap(err)
	}
	return asn, nil
}
