package core

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"storjnet/utils"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
)

func UpdateIPCompanyIfNeed(db *pg.DB, ipAddr string) (bool, error) {
	var t int64
	_, err := db.Query(&t, `
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

	company, found, err := fetchIPCompanyInfo(ipAddr)
	if err != nil {
		return false, merry.Wrap(err)
	}
	if !found {
		log.Warn().Str("ip", ipAddr).Msg("company not found")
		return false, nil
	}

	var intersections []struct {
		IPFrom, IPTo utils.NetAddrPG
		Incolumitas  ipCompanyInfoToSave
	}
	_, err = db.Query(&intersections, `
		SELECT ip_from, ip_to, incolumitas FROM network_companies WHERE ip_from <= ? AND ip_to >= ?`,
		company.Network.ipTo, company.Network.ipFrom)
	if err != nil {
		return false, merry.Wrap(err)
	}

	shouldUpdateByIPs := false
	for _, isec := range intersections {
		if isec.IPFrom == company.Network.ipFrom && isec.IPTo == company.Network.ipTo {
			shouldUpdateByIPs = true
			break
		} else {
			log.Warn().Any("old", isec).Any("new", company).Msg("companies intersection")
		}
	}

	if shouldUpdateByIPs {
		log.Debug().Str("network", company.Network.String()).Str("name", company.Name).Msg("updating company")
		_, err = db.Exec(`
			UPDATE network_companies SET incolumitas = ?, incolumitas_updated_at = NOW()
			WHERE (ip_from, ip_to) = (?, ?)`,
			company.ipCompanyInfoToSave, company.Network.ipFrom, company.Network.ipTo)
	} else {
		log.Debug().Str("network", company.Network.String()).Str("name", company.Name).Msg("inserting company")
		_, err = db.Exec(`
			INSERT INTO network_companies (ip_from, ip_to, incolumitas, incolumitas_updated_at)
			VALUES (?, ?, ?, NOW())`,
			company.Network.ipFrom, company.Network.ipTo, company.ipCompanyInfoToSave)
	}
	if err != nil {
		return false, merry.Wrap(err)
	}
	return true, nil
}

type ipNetwork struct {
	ipFrom utils.NetAddrPG
	ipTo   utils.NetAddrPG
}

func (n ipNetwork) String() string {
	return n.ipFrom.String() + " - " + n.ipTo.String()
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

		n.ipFrom.Addr = ipFrom
		n.ipTo.Addr = ipTo
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

		n.ipFrom.Addr = prefix.Masked().Addr()
		n.ipTo.Addr = netip.AddrFrom16(ipToArr).Unmap()
	}

	return nil
}

type ipCompanyInfoToSave struct {
	Name   string `json:"name"`   // "Supercom of California Limited",
	Domain string `json:"domain"` // "supercom.ca",
	Type   string `json:"type"`   // "business",
}

type ipCompanyInfo struct {
	ipCompanyInfoToSave
	Network ipNetwork `json:"network"` // "205.207.214.0 - 205.207.215.255",
	// AbuserScore string // "0 (Very Low)",
	// Whois       string // "https://api.incolumitas.com/?whois=205.207.214.0"
}

type ipInfoResponse struct {
	Company *ipCompanyInfo `json:"company"`
	Error   string         `json:"error"`
	Message string         `json:"message"`
}

func fetchIPCompanyInfo(ipAddr string) (ipCompanyInfo, bool, error) {
	if net.ParseIP(ipAddr).IsPrivate() {
		return ipCompanyInfo{}, false, nil
	}

	req, err := http.NewRequest("GET", "https://api.incolumitas.com/?q="+ipAddr, nil)
	if err != nil {
		return ipCompanyInfo{}, false, merry.Wrap(err)
	}
	httpClient := http.Client{
		Timeout: 4 * time.Second, //got some timeouts after 3 seconds
	}
	// dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:9051", nil, proxy.Direct)
	// if err != nil {
	// 	return ipCompanyInfo{}, false, merry.Wrap(err)
	// }
	// httpClient.Transport = &http.Transport{DialContext: dialer.(proxy.ContextDialer).DialContext}

	resp, err := httpClient.Do(req)
	if err != nil {
		return ipCompanyInfo{}, false, merry.Wrap(err)
	}
	defer resp.Body.Close()

	info := ipInfoResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return ipCompanyInfo{}, false, merry.Wrap(err)
	}
	if info.Error != "" {
		return ipCompanyInfo{}, false, merry.Errorf("IP %s: %s: %s", ipAddr, info.Error, info.Message)
	}

	name := "n/a"
	if info.Company != nil {
		name = info.Company.Name
	}
	log.Debug().Str("IP", ipAddr).Str("comp", name).Msg("fetched IP company from incolumitas.com")

	if info.Company == nil {
		return ipCompanyInfo{}, false, nil
	}

	return *info.Company, true, nil
}
