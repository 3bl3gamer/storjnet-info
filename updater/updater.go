package updater

import (
	"context"
	"storjnet/core"
	"storjnet/utils"
	"sync/atomic"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
	"storj.io/common/storj"
)

var ErrDialFail = merry.New("dial failed")
var ErrPingFail = merry.New("ping failed")

type UserNodeWithErr struct {
	core.UserNode
	Err error
}

func doPing(sat *utils.Satellite, node *core.Node) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stt := time.Now()
	conn, err := sat.Dial(ctx, node.Address, node.ID, utils.SatModeTCP)
	if err != nil {
		return 0, ErrDialFail.WithCause(err)
	}
	defer conn.Close()

	if node.PingMode == "ping" {
		if err := sat.Ping(ctx, conn, utils.SatModeTCP); err != nil {
			return 0, ErrPingFail.WithCause(err)
		}
	}

	return time.Now().Sub(stt), nil
}

func startOldPingNodesLoader(db *pg.DB, userNodesChan chan *core.UserNode, chunkSize int) utils.Worker {
	worker := utils.NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		for {
			userNodes := make([]*core.UserNode, chunkSize)
			err := db.RunInTransaction(func(tx *pg.Tx) error {
				_, err := tx.Query(&userNodes, `
					SELECT user_id, node_id AS raw_id, address, ping_mode FROM user_nodes
					WHERE ping_mode != 'off'
					  AND (last_pinged_at IS NULL OR last_pinged_at < NOW() - INTERVAL '0.9 minute')
					ORDER BY last_pinged_at ASC NULLS FIRST
					LIMIT ?
					FOR UPDATE`, chunkSize)
				if err != nil {
					return merry.Wrap(err)
				}
				if len(userNodes) == 0 {
					return nil
				}
				if err := core.ConvertUserNodeIDs(userNodes); err != nil {
					return merry.Wrap(err)
				}
				nodeIDs := make(storj.NodeIDList, len(userNodes))
				for i, node := range userNodes {
					nodeIDs[i] = node.ID
				}
				_, err = tx.Exec(`UPDATE user_nodes SET last_pinged_at = NOW() WHERE node_id IN (?)`, pg.In(nodeIDs))
				return merry.Wrap(err)
			})
			if err != nil {
				worker.AddError(err)
				return
			}

			log.Info().Int("IDs count", len(userNodes)).Msg("PING:OLD")
			if len(userNodes) == 0 {
				time.Sleep(10 * time.Second)
			}
			for _, node := range userNodes {
				userNodesChan <- node
			}
		}
	}()
	return worker
}

func startNodesPinger(db *pg.DB, userNodesInChan chan *core.UserNode, userNodesOutChan chan *UserNodeWithErr, routinesCount int) utils.Worker {
	worker := utils.NewSimpleWorker(routinesCount)

	sat := &utils.Satellite{}
	if err := sat.SetUp("identity"); err != nil {
		worker.AddError(err)
		return worker
	}

	stamp := time.Now().Unix()
	countTotal := int64(0)
	countOk := int64(0)
	countErrDial := int64(0)
	countErrPing := int64(0)
	countErrTotal := int64(0)
	for i := 0; i < routinesCount; i++ {
		go func() {
			defer worker.Done()
			for userNode := range userNodesInChan {
				nodeWithErr := &UserNodeWithErr{UserNode: *userNode, Err: nil}
				nodeWithErr.LastPingedAt = time.Now()

				pingDuration, err := doPing(sat, &userNode.Node)
				if err != nil {
					atomic.AddInt64(&countErrTotal, 1)
					if merry.Is(err, ErrDialFail) {
						atomic.AddInt64(&countErrDial, 1)
					} else if merry.Is(err, ErrPingFail) {
						atomic.AddInt64(&countErrPing, 1)
					}
					log.Info().Str("id", nodeWithErr.ID.String()).Msg(err.Error())
					nodeWithErr.Err = err
				} else {
					nodeWithErr.LastPing = pingDuration.Microseconds() / 1000
					nodeWithErr.LastUpAt = nodeWithErr.LastPingedAt
					atomic.AddInt64(&countOk, 1)
				}
				userNodesOutChan <- nodeWithErr

				if atomic.AddInt64(&countTotal, 1)%100 == 0 {
					log.Info().
						Int64("total", countTotal).Int64("ok", countOk).
						Int64("err", countErrTotal).Int64("errDial", countErrDial).Int64("errPing", countErrPing).
						Float64("rpm", float64(countTotal)/float64(time.Now().Unix()-stamp)*60).
						Msg("PING:GET")
				}
			}
		}()
	}
	return worker
}

func startPingedNodesSaver(db *pg.DB, userNodesChan chan *UserNodeWithErr, chunkSize int) utils.Worker {
	worker := utils.NewSimpleWorker(1)
	userNodesChanI := make(chan interface{}, 16)

	go func() {
		for node := range userNodesChan {
			userNodesChanI <- node
		}
		close(userNodesChanI)
	}()

	count := 0
	countNew := 0
	go func() {
		defer worker.Done()
		err := utils.SaveChunked(db, chunkSize, userNodesChanI, func(tx *pg.Tx, items []interface{}) error {
			ids := make([]storj.NodeID, len(items))
			for i, nodeI := range items {
				ids[i] = nodeI.(*UserNodeWithErr).ID
			}
			if _, err := tx.Exec("SELECT 1 FROM user_nodes WHERE node_id IN (?) FOR UPDATE", pg.In(ids)); err != nil {
				return merry.Wrap(err)
			}

			for _, nodeI := range items {
				node := nodeI.(*UserNodeWithErr)
				stamp := node.LastPingedAt.Unix()
				index := stamp%(24*3600)/60 + 1
				timeHint := (stamp % 60) / 4

				var pingValue int64
				if node.Err == nil {
					ping := node.LastPing
					if ping >= 2000 {
						ping = 2000 - 1
					}
					if ping <= 1 {
						ping = 2
					}
					pingValue = timeHint*2000 + ping
				} else {
					pingValue = timeHint*2000 + 1
				}

				// user_node flags and timestamps
				var err error
				if node.Err == nil {
					_, err = tx.Exec(`
						UPDATE user_nodes SET last_ping = ?, last_ping_was_ok = true, last_up_at = ?
						WHERE node_id = ? AND user_id = ?`,
						node.LastPing, node.LastUpAt, node.ID, node.UserID)
				} else {
					_, err = tx.Exec(`
						UPDATE user_nodes SET last_ping_was_ok = false
						WHERE node_id = ? AND user_id = ?`,
						node.ID, node.UserID)
				}
				if err != nil {
					return merry.Wrap(err)
				}

				// user_node auto off
				if node.Err != nil {
					/*
						res, err := tx.Exec(`
							UPDATE user_nodes SET ping_mode = 'off'
							WHERE node_id = ? AND user_id = ? AND ping_mode != 'off'
							  AND COALESCE(last_up_at, created_at) < NOW() - INTERVAL '30 days'`,
							node.ID, node.UserID)
						if err != nil {
							return merry.Wrap(err)
						}
						if res.RowsAffected() != 0 {
							log.Info().Int64("user_id", node.UserID).Str("node_id", node.ID.String()).Msg("pings turned off")
						}
					*/
					/*
						res, err := tx.Exec(`
							SELECT 1 FROM user_nodes
							WHERE node_id = ? AND user_id = ? AND ping_mode != 'off'
								AND COALESCE(last_up_at, created_at) < NOW() - INTERVAL '30 days'`,
							node.ID, node.UserID)
						if err != nil {
							return merry.Wrap(err)
						}
						if res.RowsAffected() != 0 {
							log.Info().Int64("user_id", node.UserID).Str("node_id", node.ID.String()).Msg("should turn pings off")
						}
					*/
				}

				// history (update attempt, most common)
				res, err := tx.Exec(`
					UPDATE user_nodes_history SET pings[?] = ?
					WHERE node_id = ? AND user_id = ? AND date = (? at time zone 'utc')::date
					`, index, pingValue, node.ID, node.UserID, node.LastPingedAt)
				if err != nil {
					return merry.Wrap(err)
				}
				// history (insert, in case of no updates)
				if res.RowsAffected() == 0 {
					_, err := tx.Exec(`
						INSERT INTO user_nodes_history (node_id, user_id, date, pings)
						VALUES (?, ?, (? at time zone 'utc')::date, array(SELECT CASE WHEN i = ? THEN ? ELSE 0 END FROM generate_series(1, 24*60) AS i))
						`, node.ID, node.UserID, node.LastPingedAt, index, pingValue)
					if err != nil {
						return merry.Wrap(err)
					}
					countNew++
				}
				count++
			}
			log.Info().Int("total", count).Int("new", countNew).Msg("PING:SAVE")
			return nil
		})
		log.Info().Msg("PING:SAVE:DONE")
		if err != nil {
			worker.AddError(err)
		}
	}()
	return worker
}

func StartUpdater() error {
	db := utils.MakePGConnection()
	userNodesInChan := make(chan *core.UserNode, 16)
	userNodesOutChan := make(chan *UserNodeWithErr, 16)

	workers := []utils.Worker{
		startOldPingNodesLoader(db, userNodesInChan, 16),
		startNodesPinger(db, userNodesInChan, userNodesOutChan, 16),
		startPingedNodesSaver(db, userNodesOutChan, 8),
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
