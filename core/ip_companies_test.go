package core

import (
	"encoding/json"
	"net/netip"
	"testing"
)

func Test_ipNetwork(t *testing.T) {
	tests := []struct {
		src, ipFrom, ipTo string
	}{
		{src: `"0.0.0.0 - 255.255.255.255"`, ipFrom: "0.0.0.0", ipTo: "255.255.255.255"},
		{src: `"0.1.2.3 - 1.2.3.4"`, ipFrom: "0.1.2.3", ipTo: "1.2.3.4"},

		{src: `"1.2.3.4/25"`, ipFrom: "1.2.3.0", ipTo: "1.2.3.127"},
		{src: `"1.2.3.128/25"`, ipFrom: "1.2.3.128", ipTo: "1.2.3.255"},
		{src: `"1.2.3.4/24"`, ipFrom: "1.2.3.0", ipTo: "1.2.3.255"},
		{src: `"1.2.3.4/23"`, ipFrom: "1.2.2.0", ipTo: "1.2.3.255"},

		{src: `"2001:4002:: - 2001:4003::"`, ipFrom: "2001:4002::", ipTo: "2001:4003::"},
		{src: `"2001:4002::/33"`, ipFrom: "2001:4002::", ipTo: "2001:4002:7fff:ffff:ffff:ffff:ffff:ffff"},
		{src: `"2001:4002::/32"`, ipFrom: "2001:4002::", ipTo: "2001:4002:ffff:ffff:ffff:ffff:ffff:ffff"},
		{src: `"2001:4002::/31"`, ipFrom: "2001:4002::", ipTo: "2001:4003:ffff:ffff:ffff:ffff:ffff:ffff"},
	}
	for _, test := range tests {
		var network ipNetwork
		if err := json.Unmarshal([]byte(test.src), &network); err != nil {
			t.Fatal(err)
		}
		if network.ipFrom.Compare(netip.MustParseAddr(test.ipFrom)) != 0 {
			t.Errorf("%s: %s != %s", test.src, network.ipFrom, test.ipFrom)
		}
		if network.ipTo.Compare(netip.MustParseAddr(test.ipTo)) != 0 {
			t.Errorf("%s: %s != %s", test.src, network.ipTo, test.ipTo)
		}
	}
}
