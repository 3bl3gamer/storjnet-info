package core

import (
	"encoding/json"
	"net"
	"net/netip"
	"storjnet/utils"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v10"
	"github.com/rs/zerolog/log"
)

func UpdateIPCompanyIfNeed(db *pg.DB, ipAddr string) (bool, error) {
	var t int64
	_, err := db.Query(pg.Scan(&t), `
		SELECT 1 FROM network_companies
		WHERE ip_from <= ? AND ? <= ip_to
		  AND incolumitas IS NOT NULL
		  AND incolumitas_updated_at > NOW() - INTERVAL '7 days'
		LIMIT 1`,
		ipAddr, ipAddr)
	if t == 1 {
		return false, nil
	}
	if err != nil {
		return false, merry.Wrap(err)
	}
	_, err = db.Query(pg.Scan(&t), `
		SELECT 1 FROM network_company_unknown_ips
		WHERE ip_addr = ?
		  AND updated_at > NOW() - INTERVAL '3 days'`,
		ipAddr, ipAddr)
	if t == 1 {
		return false, nil
	}
	if err != nil {
		return false, merry.Wrap(err)
	}

	company, found, err := fetchIPCompanyInfo(ipAddr)
	if err != nil {
		return false, merry.Wrap(err)
	}
	if !found {
		log.Warn().Str("ip", ipAddr).Msg("company not found, removing existing (if any)")
		res, err := db.Exec(`
			DELETE FROM network_companies WHERE ip_from <= ? AND ip_to >= ?`,
			ipAddr, ipAddr)
		if err != nil {
			return false, merry.Wrap(err)
		}
		log.Debug().Str("ip", ipAddr).Int("count", res.RowsAffected()).Msg("removed companies")

		_, err = db.Exec(`
			INSERT INTO network_company_unknown_ips (ip_addr) VALUES (?)
			ON CONFLICT (ip_addr) DO UPDATE SET updated_at = NOW()`,
			ipAddr)
		if err != nil {
			return false, merry.Wrap(err)
		}
		return false, nil
	}

	var intersections []struct {
		ID          int64
		Network     ipNetwork
		Incolumitas ipCompanyInfoToSave
	}
	_, err = db.Query(&intersections, `
		SELECT id, '"'||host(ip_from)||' - '||host(ip_to)||'"' AS network, incolumitas
		FROM network_companies WHERE ip_from <= ? AND ip_to >= ?`,
		company.Network.IPTo, company.Network.IPFrom)
	if err != nil {
		return false, merry.Wrap(err)
	}

	isAlreadySaved := false
	for _, isec := range intersections {
		// isec.range equals -> updating
		if isec.Network.IPFrom == company.Network.IPFrom && isec.Network.IPTo == company.Network.IPTo {
			if !isAlreadySaved {
				log.Debug().Any("net", company.Network).Str("name", company.Name).Msg("updating company")
				_, err := db.Exec(`
				UPDATE network_companies SET incolumitas = ?, incolumitas_updated_at = NOW() WHERE id = ?`,
					company.ipCompanyInfoToSave, isec.ID)
				isAlreadySaved = true
				if err != nil {
					return false, merry.Wrap(err)
				}
			} else {
				log.Debug().Any("net", company.Network).Any("old", isec.Network).Str("name", company.Name).Msg("removing existing double")
				_, err := db.Exec(`
					DELETE FROM network_companies WHERE id = ?`,
					isec.ID)
				if err != nil {
					return false, merry.Wrap(err)
				}
			}
			continue
		}

		// isec.range inside -> removing
		if company.Network.IPFrom.LE(isec.Network.IPFrom) && isec.Network.IPTo.LE(company.Network.IPTo) {
			if isec.Incolumitas.Equals(company.ipCompanyInfoToSave) {
				log.Debug().Any("net", company.Network).Any("old", isec.Network).Str("name", company.Name).Msg("removing existing inner")
				_, err := db.Exec(`
					DELETE FROM network_companies WHERE id = ?`,
					isec.ID)
				if err != nil {
					return false, merry.Wrap(err)
				}
				continue
			}
		}

		log.Warn().Any("old", isec).Any("new", company).Msg("companies intersection")
	}

	if !isAlreadySaved {
		log.Debug().Any("net", company.Network).Str("name", company.Name).Msg("inserting company")
		_, err := db.Exec(`
			INSERT INTO network_companies (ip_from, ip_to, incolumitas, incolumitas_updated_at)
			VALUES (?, ?, ?, NOW())`,
			company.Network.IPFrom, company.Network.IPTo, company.ipCompanyInfoToSave)
		if err != nil {
			return false, merry.Wrap(err)
		}
	}
	return true, nil
}

type ipNetwork struct {
	IPFrom utils.NetAddrPG `json:"ip_from"`
	IPTo   utils.NetAddrPG `json:"ip_to"`
}

func (n ipNetwork) String() string {
	return n.IPFrom.String() + " - " + n.IPTo.String()
}

func (n ipNetwork) MarshalJSON() ([]byte, error) {
	return []byte(`"` + n.String() + `"`), nil
}

func (n *ipNetwork) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return merry.Wrap(err)
	}

	sepIndex := strings.Index(str, "-")
	if sepIndex != -1 {
		ipFrom, err := netip.ParseAddr(strings.TrimSpace(str[:sepIndex]))
		if err != nil {
			return merry.Wrap(err)
		}
		ipTo, err := netip.ParseAddr(strings.TrimSpace(str[sepIndex+1:]))
		if err != nil {
			return merry.Wrap(err)
		}

		n.IPFrom.Addr = ipFrom
		n.IPTo.Addr = ipTo
	} else {
		prefix, err := netip.ParsePrefix(str)
		if err != nil {
			return merry.Wrap(err)
		}

		ipToArr := prefix.Addr().As16()
		prefixBits := 128 - prefix.Addr().BitLen() + prefix.Bits()
		for i := 15; i > prefixBits/8; i-- {
			ipToArr[i] = 0xFF
		}
		ipToArr[prefixBits/8] |= (^byte(0)) >> (prefixBits % 8)
		// fmt.Printf("%v (/%d) %08b %08b\n", ipToArr, prefix.Bits(), ipToArr[prefixBits/8], (^byte(0))>>(prefixBits%8))

		n.IPFrom.Addr = prefix.Masked().Addr()
		n.IPTo.Addr = netip.AddrFrom16(ipToArr).Unmap()
	}

	return nil
}

type ipCompanyInfoToSave struct {
	Name   string `json:"name"`   // "Supercom of California Limited",
	Domain string `json:"domain"` // "supercom.ca",
	Type   string `json:"type"`   // "business",
}

func (i ipCompanyInfoToSave) Equals(other ipCompanyInfoToSave) bool {
	return i.Name == other.Name && i.Domain == other.Domain && i.Type == other.Type
}

type ipCompanyInfo struct {
	ipCompanyInfoToSave
	Network ipNetwork `json:"network"` // "205.207.214.0 - 205.207.215.255",
	// AbuserScore string // "0 (Very Low)",
	// Whois       string // "https://api.ipapi.is/?whois=205.207.214.0"
}

type ipInfoResponse struct {
	Company *ipCompanyInfo `json:"company"`
}

// The company object (and so name, domain, type and network) requires an API key, see fetchIPAPI.
func fetchIPCompanyInfo(ipAddr string) (ipCompanyInfo, bool, error) {
	if net.ParseIP(ipAddr).IsPrivate() {
		return ipCompanyInfo{}, false, nil
	}

	info := ipInfoResponse{}
	//got some timeouts after 3 seconds
	if err := fetchIPAPI(ipAddr, 4*time.Second, &info); err != nil {
		return ipCompanyInfo{}, false, merry.Wrap(err)
	}

	name := "n/a"
	if info.Company != nil {
		name = info.Company.Name
	}
	log.Debug().Str("IP", ipAddr).Str("comp", name).Msg("fetched IP company from ipapi (ex incolumitas.com)")

	if info.Company == nil {
		return ipCompanyInfo{}, false, nil
	}

	return *info.Company, true, nil
}
