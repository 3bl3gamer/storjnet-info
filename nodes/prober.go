package nodes

import (
	"context"
	"storjnet/utils"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
	"storj.io/common/storj"
)

type ProbeNode struct {
	RawID  []byte
	ID     storj.NodeID
	IPAddr string
	Port   uint16
}

func probe(sat *utils.Satellite, node *ProbeNode) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	address := node.IPAddr + ":" + strconv.Itoa(int(node.Port))
	conn, err := sat.Dialer.DialNodeURL(ctx, storj.NodeURL{ID: node.ID, Address: address})
	if err != nil {
		return merry.Wrap(err)
	}
	defer conn.Close()

	return nil
}

func startOldNodesLoader(db *pg.DB, nodesChan chan *ProbeNode, chunkSize int) utils.Worker {
	worker := utils.NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		for {
			nodes := make([]*ProbeNode, chunkSize)
			err := db.RunInTransaction(func(tx *pg.Tx) error {
				_, err := tx.Query(&nodes, `
					SELECT id AS raw_id, ip_addr, port FROM nodes
					WHERE checked_at IS NULL OR checked_at < NOW() - INTERVAL '10 minutes'
					ORDER BY checked_at ASC NULLS FIRST
					LIMIT ?
					FOR UPDATE`, chunkSize)
				if err != nil {
					return merry.Wrap(err)
				}
				if len(nodes) == 0 {
					return nil
				}
				for _, node := range nodes {
					node.ID, err = storj.NodeIDFromBytes(node.RawID)
					if err != nil {
						return merry.Wrap(err)
					}
				}
				nodeIDs := make(storj.NodeIDList, len(nodes))
				for i, node := range nodes {
					nodeIDs[i] = node.ID
				}
				_, err = tx.Exec(`UPDATE nodes SET checked_at = NOW() WHERE id IN (?)`, pg.In(nodeIDs))
				return merry.Wrap(err)
			})
			if err != nil {
				worker.AddError(err)
				return
			}

			log.Info().Int("IDs count", len(nodes)).Msg("PROBE:OLD")
			if len(nodes) == 0 {
				time.Sleep(10 * time.Second)
			}
			for _, node := range nodes {
				nodesChan <- node
			}
		}
	}()
	return worker
}

func startNodesProber(db *pg.DB, nodesInChan, nodesOutChan chan *ProbeNode, routinesCount int) utils.Worker {
	worker := utils.NewSimpleWorker(routinesCount)

	sat := &utils.Satellite{}
	if err := sat.SetUp("identity"); err != nil {
		worker.AddError(err)
		return worker
	}

	stamp := time.Now().Unix()
	countTotal := int64(0)
	countOk := int64(0)
	countErr := int64(0)
	for i := 0; i < routinesCount; i++ {
		go func() {
			defer worker.Done()
			for node := range nodesInChan {
				err := probe(sat, node)
				if err != nil {
					atomic.AddInt64(&countErr, 1)
					log.Info().Str("id", node.ID.String()).Msg(err.Error())
				} else {
					atomic.AddInt64(&countOk, 1)
					nodesOutChan <- node
				}

				if atomic.AddInt64(&countTotal, 1)%1 == 0 {
					log.Info().
						Int64("total", countTotal).Int64("ok", countOk).Int64("err", countErr).
						Float64("rpm", float64(countTotal)/float64(time.Now().Unix()-stamp)*60).
						Msg("PROBE:GET")
				}
			}
		}()
	}
	return worker
}

func startPingedNodesSaver(db *pg.DB, nodesChan chan *ProbeNode, chunkSize int) utils.Worker {
	worker := utils.NewSimpleWorker(1)
	nodesChanI := make(chan interface{}, 16)

	go func() {
		for node := range nodesChan {
			nodesChanI <- node
		}
		close(nodesChanI)
	}()

	go func() {
		defer worker.Done()
		err := utils.SaveChunked(db, chunkSize, nodesChanI, func(tx *pg.Tx, items []interface{}) error {
			ids := make([]storj.NodeID, len(items))
			for i, nodeI := range items {
				ids[i] = nodeI.(*ProbeNode).ID
			}
			if _, err := tx.Exec("UPDATE nodes SET updated_at = NOW() WHERE id IN (?)", pg.In(ids)); err != nil {
				return merry.Wrap(err)
			}
			return nil
		})
		log.Info().Msg("PROBE:SAVE:DONE")
		if err != nil {
			worker.AddError(err)
		}
	}()
	return worker
}

func StartProber() error {
	db := utils.MakePGConnection()
	nodesInChan := make(chan *ProbeNode, 16)
	nodesOutChan := make(chan *ProbeNode, 16)

	workers := []utils.Worker{
		startOldNodesLoader(db, nodesInChan, 16),
		startNodesProber(db, nodesInChan, nodesOutChan, 16),
		startPingedNodesSaver(db, nodesOutChan, 1),
	}
	for {
		for _, worker := range workers {
			if err := worker.PopError(); err != nil {
				return err
			}
		}
		time.Sleep(time.Second)
	}
}
