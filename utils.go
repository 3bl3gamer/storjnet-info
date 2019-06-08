package main

import (
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abh/geoip"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
	"github.com/lib/pq"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func openFileOrStdin(fpath string) (*os.File, error) {
	if fpath == "-" {
		return os.Stdin, nil
	} else {
		f, err := os.Open(fpath)
		return f, merry.Wrap(err)
	}
}

func makePGConnection() *pg.DB {
	db := pg.Connect(&pg.Options{User: "storjinfo", Password: "storj", Database: "storjinfo_db"})
	// db.OnQueryProcessed(func(event *pg.QueryProcessedEvent) {
	// 	query, err := event.FormattedQuery()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	log.Printf("\033[36m%s \033[34m%s\033[39m", time.Since(event.StartTime), query)
	// })
	return db
}

func makeGeoIPConnection() (*geoip.GeoIP, error) {
	gdb, err := geoip.Open("/usr/share/GeoIP/GeoIPCity.dat")
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return gdb, nil
}

func saveChunked(db *pg.DB, chunkSize int, channel chan interface{}, handler func(tx *pg.Tx, items []interface{}) error) error {
	var err error
	items := make([]interface{}, 0, chunkSize)
	for item := range channel {
		items = append(items, item)
		if len(items) >= chunkSize {
			err = db.RunInTransaction(func(tx *pg.Tx) error {
				return merry.Wrap(handler(tx, items))
			})
			if err != nil {
				return merry.Wrap(err)
			}
			items = items[:0]
		}
	}
	if len(items) > 0 {
		err = db.RunInTransaction(func(tx *pg.Tx) error {
			return merry.Wrap(handler(tx, items))
		})
	}
	return merry.Wrap(err)
}

// func waitAndCheck(wg *sync.WaitGroup, errChan chan error) error {
// 	wg.Wait()
// 	select {
// 	case err := <-errChan:
// 		return err
// 		return nil
// 	}
// }

type Worker interface {
	Done()
	AddError(error)
	PopError() error
	CloseAndWait() error
}

type SimpleWorker struct {
	errChan  chan error
	doneChan chan struct{}
}

func NewSimpleWorker(count int) *SimpleWorker {
	return &SimpleWorker{
		errChan:  make(chan error, 1),
		doneChan: make(chan struct{}, count),
	}
}

func (w SimpleWorker) Done() {
	w.doneChan <- struct{}{}
}

func (w SimpleWorker) AddError(err error) {
	w.errChan <- merry.WrapSkipping(err, 1)
}

func (w SimpleWorker) PopError() error {
	select {
	case err := <-w.errChan:
		return err
	default:
		return nil
	}
}

func (w SimpleWorker) CloseAndWait() error {
	for i := 0; i < cap(w.doneChan); i++ {
		<-w.doneChan
	}
	close(w.doneChan)
	close(w.errChan)
	return w.PopError()
}

type NodeIDExt storj.NodeID

func (id *NodeIDExt) Scan(val interface{}) error {
	var idBytes [32]byte
	_, err := hex.Decode(idBytes[:], val.([]byte)[2:])
	if err != nil {
		return merry.Wrap(err)
	}
	idVal, err := storj.NodeIDFromBytes(idBytes[:])
	if err != nil {
		return merry.Wrap(err)
	}
	*id = NodeIDExt(idVal)
	return nil
}

func (id NodeIDExt) String() string {
	return storj.NodeID(id).String()
}

type NodeInfoExt struct {
	ID   storj.NodeID
	Info *pb.NodeInfoResponse
}

type NodeLocation struct {
	Country   string
	City      string
	Longitude float32
	Latitude  float32
}

func (l *NodeLocation) Value() (driver.Value, error) {
	if l == nil {
		return nil, nil
	}
	// composite types seem not supported in go-pg, so there is manual formatting with semi-hacky escaping
	return fmt.Sprintf("(%s,%s,%f,%f)",
		pq.QuoteIdentifier(l.Country), pq.QuoteIdentifier(l.City), l.Longitude, l.Latitude), nil
}

type KadDataExt struct {
	Node     *pb.Node
	Location *NodeLocation `sql:"composite:node_Location"`
}

func scanCompositePaitStr(val interface{}) (string, string, string, error) {
	bytes, ok := val.([]byte)
	if !ok {
		return "", "", "", merry.Errorf("expected value to be []byte, got %#v", val)
	}
	str := string(bytes)
	sepPos := strings.LastIndex(str, ",")
	if sepPos == -1 {
		return "", "", "", merry.New("can not fint two values in " + str)
	}
	return str, str[1:sepPos], str[sepPos+1 : len(str)-1], nil
}

func scanCompositeItemInt64(itemStr, str string) (int64, error) {
	val, err := strconv.ParseInt(itemStr, 10, 64)
	if err != nil {
		return 0, merry.Errorf("wrong int '%s' in %s", itemStr, str)
	}
	return val, nil
}

func scanCompositeItemFloat64(itemStr, str string) (float64, error) {
	val, err := strconv.ParseFloat(itemStr, 64)
	if err != nil {
		return 0, merry.Errorf("wrong float '%s' in %s", itemStr, str)
	}
	return val, nil
}

func scanComposite2ii(val interface{}) (int64, int64, error) {
	// "(123,234)"
	str, aStr, bStr, err := scanCompositePaitStr(val)
	if err != nil {
		return 0, 0, err
	}
	a, err := scanCompositeItemInt64(aStr, str)
	if err != nil {
		return 0, 0, err
	}
	b, err := scanCompositeItemInt64(bStr, str)
	if err != nil {
		return 0, 0, err
	}
	return a, b, nil
}

func scanComposite2fi(val interface{}) (float64, int64, error) {
	// "(12.3,234)"
	str, aStr, bStr, err := scanCompositePaitStr(val)
	if err != nil {
		return 0, 0, err
	}
	a, err := scanCompositeItemFloat64(aStr, str)
	if err != nil {
		return 0, 0, err
	}
	b, err := scanCompositeItemInt64(bStr, str)
	if err != nil {
		return 0, 0, err
	}
	return a, b, nil
}

func scanComposite2si(val interface{}) (string, int64, error) {
	// "(12.3,234)"
	str, aStr, bStr, err := scanCompositePaitStr(val)
	if err != nil {
		return "", 0, err
	}
	a, err := unescapePGString(aStr)
	if err != nil {
		return "", 0, err
	}
	b, err := scanCompositeItemInt64(bStr, str)
	if err != nil {
		return "", 0, err
	}
	return a, b, nil
}

type ActivityStatItem struct {
	Hours, Count int64
}
type ActivityStatItems []ActivityStatItem

func (i *ActivityStatItem) Scan(val interface{}) (err error) {
	i.Hours, i.Count, err = scanComposite2ii(val)
	return merry.Wrap(err)
}

func (items ActivityStatItems) At(hour int64) int64 {
	for _, item := range items {
		if item.Hours == hour {
			return item.Count
		}
	}
	return 0
}

type DataStatItem struct {
	Percentile float64
	BytesCount int64
}
type DataStatItems []DataStatItem

func (i *DataStatItem) Scan(val interface{}) (err error) {
	i.Percentile, i.BytesCount, err = scanComposite2fi(val)
	return merry.Wrap(err)
}

func (items DataStatItems) At(perc float64) int64 {
	for _, item := range items {
		if math.Abs(item.Percentile-perc) < 0.0005 {
			return item.BytesCount
		}
	}
	return 0
}

type VersionStatItem struct {
	Version string
	Count   int64
}
type VersionStatItems []VersionStatItem

func (i *VersionStatItem) Scan(val interface{}) (err error) {
	i.Version, i.Count, err = scanComposite2si(val)
	return merry.Wrap(err)
}

type CountryStatItem struct {
	Country string
	Count   int64
}
type CountryStatItems []CountryStatItem

func (items CountryStatItems) Top(n int64) []CountryStatItem {
	res := make([]CountryStatItem, 0, n)
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.Country != "" {
			res = append(res, item)
		}
		if int64(len(res)) >= n {
			break
		}
	}
	return res
}

func (items CountryStatItems) UnknownCount() int64 {
	for _, item := range items {
		if item.Country == "" {
			return item.Count
		}
	}
	return 0
}

func (i *CountryStatItem) Scan(val interface{}) (err error) {
	i.Country, i.Count, err = scanComposite2si(val)
	return merry.Wrap(err)
}

type DifficultyStatItem struct {
	Difficulty, Count int64
}
type DifficultyStatItems []DifficultyStatItem

func (i *DifficultyStatItem) Scan(val interface{}) (err error) {
	i.Difficulty, i.Count, err = scanComposite2ii(val)
	return merry.Wrap(err)
}

type TypesStatItem struct {
	Type, Count int64
}
type TypesStatItems []TypesStatItem

func (items TypesStatItems) OfType(nodeType pb.NodeType) int64 {
	for _, item := range items {
		if item.Type == int64(nodeType) {
			return item.Count
		}
	}
	return 0
}

func (i *TypesStatItem) Scan(val interface{}) (err error) {
	i.Type, i.Count, err = scanComposite2ii(val)
	return merry.Wrap(err)
}

func (i *TypesStatItem) TypeString() string {
	return pb.NodeType_name[int32(i.Type)]
}

type GlobalStat struct {
	ID            int64
	CreatedAt     time.Time
	CountTotal    int64
	CountHours    ActivityStatItems   `sql:",array"`
	FreeDisk      DataStatItems       `sql:",array"`
	FreeDiskTotal DataStatItems       `sql:",array"`
	FreeBandwidth DataStatItems       `sql:",array"`
	Versions      VersionStatItems    `sql:",array"`
	Countries     CountryStatItems    `sql:",array"`
	Difficulties  DifficultyStatItems `sql:",array"`
	Types         TypesStatItems      `sql:",array"`
}

func nextChar(str string, pos *int) (byte, error) {
	if len(str) <= *pos {
		return 0, merry.Errorf("buffer suddenly ended: %d, %s", *pos, str)
	}
	c := str[*pos]
	*pos++
	return c, nil
}

func unescapePGString(str string) (string, error) {
	if len(str) == 0 || str[0] != '"' {
		return str, nil
	}

	var destBuf []byte
	pos := 1

	c, err := nextChar(str, &pos)
	if err != nil {
		return "", err
	}

	for {
		if c == '"' {
			return string(destBuf), nil
		}

		next, err := nextChar(str, &pos)
		if err != nil {
			return string(destBuf), err
		}

		if c == '\\' && (next == '\\' || next == '"') {
			destBuf = append(destBuf, next)
			c, err = nextChar(str, &pos)
			if err != nil {
				return "", err
			}
			continue
		}

		destBuf = append(destBuf, c)
		c = next
	}

	return string(destBuf), nil
}
