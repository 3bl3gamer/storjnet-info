package main

import (
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abh/geoip"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/types"
	"github.com/gogo/protobuf/jsonpb"
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

func appendFileString(fpath, text string) error {
	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return merry.Wrap(err)
	}
	defer f.Close()
	if _, err = f.WriteString(text); err != nil {
		return merry.Wrap(err)
	}
	return nil
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

// лучше бы тут сделать структуру с анонимным полем аналогично NodeKadExt,
// но пока https://github.com/go-pg/pg/issues/1237
type NodeIDExt struct {
	storj.NodeID
}

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
	*id = NodeIDExt{idVal}
	return nil
}

func (id NodeIDExt) Value() (driver.Value, error) {
	return id.NodeID[:], nil
}

// func (id NodeIDExt) String() string {
// 	return storj.NodeID(id).String()
// }

type NodeKadExt struct {
	pb.Node
}

func (node *NodeKadExt) Scan(val interface{}) error {
	return merry.Wrap(jsonpb.UnmarshalString(string(val.([]byte)), &node.Node))
}

type SelfUpdate_Kad struct {
	ID        NodeIDExt
	KadParams NodeKadExt
}

type SelfUpdate_Self struct {
	SelfUpdate_Kad
	SelfParams    *pb.NodeInfoResponse
	SelfUpdateErr error
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
		escapePGString(l.Country), escapePGString(l.City), l.Longitude, l.Latitude), nil
}

type KadDataExt struct {
	Node     *pb.Node
	Location *NodeLocation `sql:"composite:node_Location"`
}

func scanCompositePairStr(val interface{}) (string, string, string, error) {
	bytes, ok := val.([]byte)
	if !ok {
		return "", "", "", merry.Errorf("expected value to be []byte, got %#v", val)
	}
	str := string(bytes)
	sepPos := strings.LastIndex(str, ",")
	if sepPos == -1 {
		return "", "", "", merry.New("can not find two values in " + str)
	}
	return str, str[1:sepPos], str[sepPos+1 : len(str)-1], nil
}
func scanCompositeTrioStr(val interface{}) (string, string, string, string, error) {
	bytes, ok := val.([]byte)
	if !ok {
		return "", "", "", "", merry.Errorf("expected value to be []byte, got %#v", val)
	}
	str := string(bytes)
	sepPos1 := strings.LastIndex(str, ",")
	if sepPos1 == -1 {
		return "", "", "", "", merry.New("can not find two values in " + str)
	}
	sepPos0 := strings.LastIndex(str[:sepPos1], ",")
	if sepPos1 == -1 {
		return "", "", "", "", merry.New("can not find three values in " + str)
	}
	return str, str[1:sepPos0], str[sepPos0+1 : sepPos1], str[sepPos1+1 : len(str)-1], nil
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
	str, aStr, bStr, err := scanCompositePairStr(val)
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
	str, aStr, bStr, err := scanCompositePairStr(val)
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
	str, aStr, bStr, err := scanCompositePairStr(val)
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

type Node struct {
	ID            storj.NodeID
	CreatedAt     time.Time
	KadParams     pb.Node
	SelfParams    *pb.NodeInfoResponse
	KadUpdatedAt  time.Time
	SelfUpdatedAt time.Time
	Location      *NodeLocation `sql:"composite:node_Location"`
}

type DataHistoryItem struct {
	Stamp         time.Time
	FreeDisk      int64
	FreeBandwidth int64
}
type DataHistoryItems []DataHistoryItem

type DataHistoryItemsSeparated struct {
	Stamps        []int64 `json:"stamps"`
	FreeDisk      []int64 `json:"freeDisk"`
	FreeBandwidth []int64 `json:"freeBandwidth"`
}

func (i *DataHistoryItem) Scan(val interface{}) error {
	str, a, b, c, err := scanCompositeTrioStr(val)
	if err != nil {
		return err
	}
	a, err = unescapePGString(a)
	if err != nil {
		return err
	}
	i.Stamp, err = types.ParseTimeString(a)
	if err != nil {
		return err
	}
	i.FreeDisk, err = scanCompositeItemInt64(b, str)
	if err != nil {
		return err
	}
	i.FreeBandwidth, err = scanCompositeItemInt64(c, str)
	if err != nil {
		return err
	}
	return nil
}

func (items DataHistoryItems) SeparatedCompact() *DataHistoryItemsSeparated {
	res := &DataHistoryItemsSeparated{
		Stamps:        make([]int64, 0, len(items)/2),
		FreeDisk:      make([]int64, 0, len(items)/2),
		FreeBandwidth: make([]int64, 0, len(items)/2),
	}
	for _, item := range items {
		// почему-то данные по объёмам в течение примерно часа приходят одни и те же
		if len(res.Stamps) == 0 || item.FreeDisk != res.FreeDisk[len(res.FreeDisk)-1] && item.FreeBandwidth != res.FreeBandwidth[len(res.FreeBandwidth)-1] {
			res.Stamps = append(res.Stamps, item.Stamp.Unix())
			res.FreeDisk = append(res.FreeDisk, item.FreeDisk)
			res.FreeBandwidth = append(res.FreeBandwidth, item.FreeBandwidth)
		}
	}
	return res
}

type NodeHistory struct {
	tableName           struct{} `sql:"nodes_history"`
	ID                  NodeIDExt
	MonthDate           time.Time
	FreeDataItems       DataHistoryItems `pg:",array"`
	ActivityStamps      []int64          `pg:",array"`
	LastSelfParamsError string
}

func nextChar(str string, pos *int) (byte, error) {
	if len(str) <= *pos {
		return 0, merry.Errorf("buffer suddenly ended: %d, %s", *pos, str)
	}
	c := str[*pos]
	*pos++
	return c, nil
}

func escapePGString(name string) string {
	return `"` + strings.Replace(name, `"`, `""`, -1) + `"`
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

var maxLogTagLen int

func logMsg(level, tag, msg string, args ...interface{}) {
	if maxLogTagLen < len(tag) {
		maxLogTagLen = len(tag)
	}
	offset := ""
	for i := len(tag); i < maxLogTagLen; i++ {
		offset += " "
	}
	msg = level + ": " + tag + ": " + offset + msg
	if len(args) == 0 {
		log.Print(msg)
	} else {
		log.Printf(msg, args...)
	}
}
func logInfo(tag, msg string, args ...interface{}) {
	logMsg("INFO", tag, msg, args...)
}
func logWarn(tag, msg string, args ...interface{}) {
	logMsg("WARN", tag, msg, args...)
}
func logErr(tag, msg string, args ...interface{}) {
	logMsg("ERRO", tag, msg, args...)
}
