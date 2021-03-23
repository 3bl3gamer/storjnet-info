package nodes

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"storjnet/utils"
	"time"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog/log"
)

const LastFPathLabel = "<last>"

func SaveLocsSnapshot() error {
	now := time.Now()
	db := utils.MakePGConnection()

	var locs []struct{ Lon, Lat float64 }
	_, err := db.Query(&locs, `
		SELECT (location->'longitude')::float AS lon, (location->'latitude')::float AS lat
		FROM nodes
		WHERE updated_at > NOW() - INTERVAL '12 hours'
		  AND location IS NOT NULL
		ORDER BY id`)
	if err != nil {
		return merry.Wrap(err)
	}

	buf := make([]byte, 24+len(locs)*4)
	for i, loc := range locs {
		lon := uint16((loc.Lon + 180) / 360 * 0x10000)
		lat := uint16((loc.Lat + 90) / 180 * 0x10000)
		buf[24+i*4+0] = uint8(lon & 0xFF)
		buf[24+i*4+1] = uint8(lon >> 8)
		buf[24+i*4+2] = uint8(lat & 0xFF)
		buf[24+i*4+3] = uint8(lat >> 8)
	}
	binary.LittleEndian.PutUint64(buf, ^uint64(0))
	binary.LittleEndian.PutUint64(buf[8:], uint64(now.Unix()))
	binary.LittleEndian.PutUint64(buf[16:], uint64(len(locs)))

	ex, err := os.Executable()
	if err != nil {
		return merry.Wrap(err)
	}
	exPath := filepath.Dir(ex)
	locDir := exPath + "/history/locations"
	if err := os.MkdirAll(locDir, os.ModePerm); err != nil {
		return merry.Wrap(err)
	}
	fpath := locDir + "/" + now.Format("2006-01-02") + ".bin"
	fd, err := os.OpenFile(fpath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return merry.Wrap(err)
	}
	defer fd.Close()
	if _, err := fd.Write(buf); err != nil {
		return merry.Wrap(err)
	}
	return merry.Wrap(fd.Close())
}

func PrintLocsSnapshot(fpath string) error {
	log.Info().Msg("starting")

	if fpath == LastFPathLabel {
		ex, err := os.Executable()
		if err != nil {
			return merry.Wrap(err)
		}
		exPath := filepath.Dir(ex)
		locDir := exPath + "/history/locations"
		fnames, err := filepath.Glob(locDir + "/*.bin")
		if err != nil {
			return merry.Wrap(err)
		}
		sort.Strings(fnames)
		if len(fnames) == 0 {
			log.Warn().Msg("no files found in default folder " + locDir)
		}
		fpath = fnames[len(fnames)-1]
	}

	fd, err := os.Open(fpath)
	if err != nil {
		return merry.Wrap(err)
	}
	defer fd.Close()

	buf, err := io.ReadAll(fd)
	if err != nil {
		return merry.Wrap(err)
	}
	if len(buf) == 0 {
		log.Warn().Msg("file is empty")
		return nil
	}

	pos := 0
	chunksCount := 0
	for {
		b := buf[pos:]
		if len(b) == 0 {
			break
		}
		if len(b) < 24 {
			log.Warn().Msgf("expected 24 more bytes in file, got %d, exiting", len(b))
			break
		}

		prefix := binary.LittleEndian.Uint64(b)
		pos += 8
		if prefix != ^uint64(0) {
			log.Warn().Msgf("expected prefix at pos %d, found %X, skipping", pos, prefix)
			continue
		}

		stamp := time.Unix(int64(binary.LittleEndian.Uint64(b[8:])), 0)
		fmt.Printf(" === %s === \n", stamp.Format("2006-01-02 15:04:05 -0700"))
		pos += 8

		count := int(binary.LittleEndian.Uint64(b[16:]))
		fmt.Printf(" -- x%d -- \n", count)
		pos += 8
		for i := 0; i < count; i++ {
			loni := (int64(b[24+i*4+1]) << 8) + int64(b[24+i*4+0])
			lati := (int64(b[24+i*4+3]) << 8) + int64(b[24+i*4+2])
			lon := (float64(loni)/0x10000)*360 - 180
			lat := (float64(lati)/0x10000)*180 - 90
			fmt.Printf("loc: %8.3f %7.3f\n", lon, lat)
		}
		pos += count * 4
		chunksCount++
	}

	log.Info().Int("chunks", chunksCount).Msg("done")
	return nil
}
