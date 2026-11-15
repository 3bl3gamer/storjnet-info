package core

import "testing"

func Test_SatelliteGroup(t *testing.T) {
	tests := []struct {
		name, group string
	}{
		{name: "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us1.storj.io:7777", group: "official"},
		{name: "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777", group: "official"},
		{name: "1wFTAgs9DP5RSnCqKV1eLf6N9wtk4EAtmN5DpSxcs8EjT69tGE@saltlake.tardigrade.io:7777", group: "official"},
		{name: "118UWpMCHzs6CvSgWd9BfFVjw5K9pZbJjkfZJexMtSkmKxvvAW@satellite.stefan-benten.de:7777", group: "official"},
		{name: "ap1.storj.io", group: "official"},
		{name: "EU1.Storj.io:7777", group: "official"},

		{name: "1yjgPDnv1XRSh6emyQA2VZP1nquivTN8usC2BkFYDL2BbGB2nH@sj.storkspace.net:1111", group: "storkspace"},
		{name: "sj.storkspace.net", group: "storkspace"},

		// not in the list: separate groups, must not be mixed with the known ones
		{name: "12kK7Y15fU1b5YADDQiBj2Dd4WsF1LARVWHpW6nXRcusV3wiRZD@tutor.storkspace.net:7777", group: "tutor.storkspace.net"},
		{name: "fakestorj.io", group: "fakestorj.io"},
		{name: "storj.io", group: "storj.io"},
		{name: "storj.io.example.com:7777", group: "storj.io.example.com"},
	}
	for _, test := range tests {
		if group := SatelliteGroup(test.name); group != test.group {
			t.Errorf("%s: expected group %q, got %q", test.name, test.group, group)
		}
	}
}

func Test_IsTestSatellite(t *testing.T) {
	tests := map[string]bool{
		"12kK7Y15fU1b5YADDQiBj2Dd4WsF1LARVWHpW6nXRcusV3wiRZD@tutor.storkspace.net:7777": true,
		"Tutor.Storkspace.net": true,
		"1yjgPDnv1XRSh6emyQA2VZP1nquivTN8usC2BkFYDL2BbGB2nH@sj.storkspace.net:1111": false,
		"us1.storj.io:7777": false,
	}
	for name, expected := range tests {
		if res := IsTestSatellite(name); res != expected {
			t.Errorf("%s: expected %v, got %v", name, expected, res)
		}
	}
}

func Test_SatGroups(t *testing.T) {
	groups := SatGroups()
	if len(groups) != 2 || groups[0] != "official" || groups[1] != "storkspace" {
		t.Errorf("unexpected groups: %v", groups)
	}
}

func Test_SatUsernames(t *testing.T) {
	names := SatUsernames()
	if len(names) != 2 || names[0] != "satellites.official" || names[1] != "satellites.storkspace" {
		t.Errorf("unexpected usernames: %v", names)
	}
}

func Test_SatelliteIDAndHost(t *testing.T) {
	name := "1yjgPDnv1XRSh6emyQA2VZP1nquivTN8usC2BkFYDL2BbGB2nH@sj.storkspace.net:1111"
	id, host, err := SatelliteIDAndHost(name)
	if err != nil {
		t.Fatal(err)
	}
	if id.String() != "1yjgPDnv1XRSh6emyQA2VZP1nquivTN8usC2BkFYDL2BbGB2nH" || host != "sj.storkspace.net" {
		t.Errorf("wrong id/host: %s, %s", id, host)
	}

	if _, _, err := SatelliteIDAndHost("sj.storkspace.net:1111"); err == nil {
		t.Error("expected an error for a name without id")
	}
}
