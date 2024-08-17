//go:build exclude

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/ansel1/merry"
)

func main() {
	if err := mainErr(); err != nil {
		log.Fatal(merry.Details(err))
	}
}

func mainErr() error {
	resp, err := http.Get("https://raw.githubusercontent.com/stefangabos/world_countries/master/data/countries/_combined/world.json")
	if err != nil {
		return merry.Wrap(err)
	}
	var items []*struct{ Alpha2, Alpha3, EN, RU string }
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return merry.Wrap(err)
	}

	for _, item := range items {
		if strings.HasSuffix(item.EN, " of") {
			index := strings.LastIndex(item.EN, ",")
			item.EN = item.EN[:index]
		}
		if item.Alpha3 == "twn" {
			item.EN = "Taiwan"
			item.RU = "Тайвань"
		}
		if item.Alpha3 == "prk" {
			item.EN = "North Korea"
			item.RU = "Северная Корея"
		}
		if item.Alpha3 == "kor" {
			item.EN = "South Korea"
			item.RU = "Южная Корея"
		}
		if item.Alpha3 == "vnm" {
			item.EN = "Vietnam"
		}
		if item.Alpha3 == "usa" {
			item.EN = "United States"
		}
		if item.Alpha3 == "gbr" {
			item.EN = "United Kingdom"
		}
		if item.Alpha3 == "rus" {
			item.EN = "Russia"
		}
		if item.Alpha3 == "chn" {
			item.RU = "Китай"
		}
	}

	s := "package utils\nvar Countries = []*Country{\n"
	for _, item := range items {
		s += fmt.Sprintf("{A2: `%s`, A3: `%s`, EN: `%s`, RU: `%s`},\n",
			item.Alpha2, item.Alpha3, item.EN, item.RU)
	}
	s += "}"

	f, err := os.Create("countries_generated.go")
	if err != nil {
		return merry.Wrap(err)
	}
	if _, err := f.Write([]byte(s)); err != nil {
		return merry.Wrap(err)
	}
	if err := f.Close(); err != nil {
		return merry.Wrap(err)
	}
	return nil
}
