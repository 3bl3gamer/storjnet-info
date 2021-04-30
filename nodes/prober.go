package nodes

import (
	"context"
	"fmt"
	"storjnet/utils"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
	"storj.io/common/storj"
)

const (
	nodesUpdateInterval = `INTERVAL '8 minutes'`
	noNodesPauseDuraton = 30 * time.Second
	probeRoutinesCount  = 16
)

type ProbeNode struct {
	RawID  []byte
	ID     storj.NodeID
	IPAddr string
	Port   uint16
}
type ProbeNodeErr struct {
	Node    *ProbeNode
	TCPErr  error
	QUICErr error
}

func errIsKnown(err error) bool {
	msg := err.Error()
	return strings.HasPrefix(msg, "rpc: context deadline exceeded") ||
		(strings.HasPrefix(msg, "rpc: dial tcp ") &&
			(strings.Contains(msg, ": connect: connection refused") ||
				strings.Contains(msg, ": connect: no route to host") ||
				strings.Contains(msg, ": i/o timeout"))) ||
		strings.HasPrefix(msg, "rpc: tls peer certificate verification error: tlsopts error: peer ID did not match requested ID")
}

func probeWithTimeout(sat *utils.Satellite, nodeID storj.NodeID, address string, mode utils.SatMode) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return merry.Wrap(sat.DialAndClose(ctx, address, nodeID, mode))
}
func probe(sat *utils.Satellite, node *ProbeNode) (tcpErr error, quicErr error) {
	address := node.IPAddr + ":" + strconv.Itoa(int(node.Port))
	wg := sync.WaitGroup{}

	wg.Add(2)
	go func() {
		tcpErr = probeWithTimeout(sat, node.ID, address, utils.SatModeTCP)
		wg.Done()
	}()
	go func() {
		quicErr = probeWithTimeout(sat, node.ID, address, utils.SatModeQUIC)
		wg.Done()
	}()

	wg.Wait()
	return
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
					WHERE checked_at IS NULL
					   OR (checked_at < NOW() - `+nodesUpdateInterval+`
					       AND greatest(updated_at, last_received_from_sat_at) > NOW() - INTERVAL '7 days')
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
				time.Sleep(noNodesPauseDuraton)
			}
			for _, node := range nodes {
				nodesChan <- node
			}
		}
	}()
	return worker
}

func startNodesProber(db *pg.DB, nodesInChan chan *ProbeNode, nodesOutChan chan *ProbeNodeErr, routinesCount int) utils.Worker {
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
				tcpErr, quicErr := probe(sat, node)
				if tcpErr != nil && quicErr != nil {
					atomic.AddInt64(&countErr, 1)
					if !errIsKnown(tcpErr) {
						log.Info().Str("id", node.ID.String()).Msg(tcpErr.Error())
					}
				} else {
					atomic.AddInt64(&countOk, 1)
					nodesOutChan <- &ProbeNodeErr{Node: node, TCPErr: tcpErr, QUICErr: quicErr}
				}

				if atomic.AddInt64(&countTotal, 1)%100 == 0 {
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

func startPingedNodesSaver(db *pg.DB, nodesChan chan *ProbeNodeErr, chunkSize int) utils.Worker {
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
			tcpErrIDs := make([]storj.NodeID, 0, len(items))
			quicErrIDs := make([]storj.NodeID, 0, len(items))
			for i, nodeI := range items {
				n := nodeI.(*ProbeNodeErr)
				ids[i] = n.Node.ID
				if n.TCPErr != nil {
					tcpErrIDs = append(tcpErrIDs, n.Node.ID)
				}
				if n.QUICErr != nil {
					quicErrIDs = append(quicErrIDs, n.Node.ID)
				}
			}
			fmt.Println(ids, tcpErrIDs, quicErrIDs)
			_, err := tx.Exec(`
				UPDATE nodes SET
					updated_at = NOW(),
					tcp_updated_at = CASE WHEN id = any(ARRAY[?]::bytea[]) THEN tcp_updated_at ELSE NOW() END,
					quic_updated_at = CASE WHEN id = any(ARRAY[?]::bytea[]) THEN quic_updated_at ELSE NOW() END
				WHERE id IN (?)`,
				pg.In(tcpErrIDs), pg.In(quicErrIDs), pg.In(ids))
			if err != nil {
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
	nodesInChan := make(chan *ProbeNode, 32)
	nodesOutChan := make(chan *ProbeNodeErr, 32)

	workers := []utils.Worker{
		startOldNodesLoader(db, nodesInChan, 128),
		startNodesProber(db, nodesInChan, nodesOutChan, probeRoutinesCount),
		startPingedNodesSaver(db, nodesOutChan, 32),
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
