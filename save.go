package main

import (
	"log"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
	"storj.io/storj/pkg/pb"
)

type KadDataSaver struct {
	kadDataChan chan interface{}
	errChan     chan error
	doneChan    chan struct{}
	hasStopped  bool
	Count       int64
	CountNew    int64
}

func (s KadDataSaver) Add(node *pb.Node) {
	s.kadDataChan <- node
}

func (s *KadDataSaver) Stop() {
	if !s.hasStopped {
		s.hasStopped = true
		close(s.kadDataChan)
	}
}

func (s *KadDataSaver) StopAndWait() error {
	s.Stop()
	<-s.doneChan
	select {
	case err := <-s.errChan:
		return merry.Wrap(err)
	default:
		return nil
	}
}

func StartNodesKadDataSaving(db *pg.DB) *KadDataSaver {
	saver := &KadDataSaver{
		kadDataChan: make(chan interface{}, 16),
		errChan:     make(chan error, 1),
		doneChan:    make(chan struct{}, 1),
	}

	go func() {
		defer func() {
			saver.doneChan <- struct{}{}
			close(saver.doneChan)
			close(saver.errChan)
		}()
		err := saveChunked(db, 10, saver.kadDataChan, func(tx *pg.Tx, items []interface{}) error {
			for _, node := range items {
				var xmax string
				_, err := db.QueryOne(&xmax, `
				INSERT INTO storj3_nodes (id, kad_params, kad_updated_at)
				VALUES (?, ?, NOW())
				ON CONFLICT (id) DO UPDATE SET kad_params = EXCLUDED.kad_params, kad_updated_at = NOW()
				RETURNING xmax`, node.(*pb.Node).Id, node)
				if err != nil {
					return merry.Wrap(err)
				}
				saver.Count++
				if xmax == "0" {
					saver.CountNew++
				}
			}
			log.Printf("imported %d kad nodes, %d new", saver.Count, saver.CountNew)
			return nil
		})
		log.Printf("done, imported %d kad nodes, %d new", saver.Count, saver.CountNew)
		if err != nil {
			saver.errChan <- err
		}
	}()

	return saver
}
