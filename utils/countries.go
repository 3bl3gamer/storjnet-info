package utils

import "strings"

//go:generate go run countries_generator.go
//go:generate gofmt -w countries_generated.go

type Country struct{ A2, A3, EN, RU string }

var CountryA2ToA3 map[string]string
var CountryByA3 map[string]*Country

func init() {
	CountryA2ToA3 = make(map[string]string, len(Countries))
	CountryByA3 = make(map[string]*Country, len(Countries))
	for _, c := range Countries {
		CountryA2ToA3[c.A2] = c.A3
		CountryByA3[c.A3] = c
	}
}

func CountryA2ToA3IfExists(maybeA2 string) string {
	a3, ok := CountryA2ToA3[strings.ToLower(maybeA2)]
	if ok {
		return a3
	}
	return maybeA2
}

func CountryA3ToName(maybeA3, lang string) (string, bool) {
	country, ok := CountryByA3[strings.ToLower(maybeA3)]
	if ok {
		if lang == "ru" {
			return country.RU, true
		}
		return country.EN, true
	}
	return maybeA3, false
}
