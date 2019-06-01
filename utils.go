package main

import (
	"os"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
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

type NodeInfoWithID struct {
	ID   storj.NodeID
	Info *pb.NodeInfoResponse
}

// 	m := jsonpb.Marshaler{Indent: "  ", EmitDefaults: true}
// 	formatted, err := m.MarshalToString(unformatted)
// 	if err != nil {
// 		fmt.Println("Error", err)
// 		os.Exit(1)
// 	}
// 	return formatted
// }
