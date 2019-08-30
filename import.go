package main

import (
	"bufio"
	"io"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/gogo/protobuf/jsonpb"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func ImportNodeIDs(fpath string) (err error) {
	f, err := openFileOrStdin(fpath)
	if err != nil {
		return merry.Wrap(err)
	}
	defer f.Close()

	db := makePGConnection()
	nodeIDsChan := make(chan interface{}, 16)
	errChan := make(chan error, 1)

	go func() {
		lineF := bufio.NewReader(f)
		for {
			line, err := lineF.ReadString('\n')
			if err == io.EOF {
				if line == "" {
					break
				}
			} else if err != nil {
				errChan <- merry.Wrap(err)
				break
			}
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			id, err := storj.NodeIDFromString(line)
			if err != nil {
				errChan <- merry.Wrap(err)
				break
			}
			nodeIDsChan <- id
		}
		close(nodeIDsChan)
	}()

	count := 0
	countNew := 0
	err = saveChunked(db, 100, nodeIDsChan, func(tx *pg.Tx, items []interface{}) error {
		for _, id := range items {
			res, err := db.Exec(`INSERT INTO nodes (id) VALUES (?) ON CONFLICT (id) DO NOTHING RETURNING xmax`, id)
			if err != nil {
				return merry.Wrap(err)
			}
			count++
			if res.RowsAffected() > 0 {
				countNew++
			}
		}
		logInfo("IDS-IMPORT", "imported %d IDs, %d new", count, countNew)
		return nil
	})
	if err != nil {
		return merry.Wrap(err)
	}

	select {
	case err := <-errChan:
		return merry.Wrap(err)
	default:
	}

	logInfo("IDS-IMPORT", "done")
	return nil
}

func ImportNodesKadData(fpath string) (err error) {
	db := makePGConnection()
	gdb, err := makeGeoIPConnection()
	if err != nil {
		return merry.Wrap(err)
	}

	rawKadDataChan := make(chan *pb.Node, 16)
	extKadDataChan := make(chan *KadDataExt, 16)
	importer := StartNodesKadDataImporter(fpath, false, rawKadDataChan, 0)
	location := StartLocationSearcher(gdb, rawKadDataChan, extKadDataChan)
	saver := StartNodesKadDataSaver(db, extKadDataChan, 100)

	if err := importer.CloseAndWait(); err != nil {
		return merry.Wrap(err)
	}
	close(rawKadDataChan)
	if err := location.CloseAndWait(); err != nil {
		return merry.Wrap(err)
	}
	close(extKadDataChan)
	if err := saver.CloseAndWait(); err != nil {
		return merry.Wrap(err)
	}

	logInfo("KAD-IMPORT", "done")
	return nil
}

func StartNodesKadDataImporter(fpath string, infinite bool, kadDataChan chan *pb.Node, recentSkipSize int) Worker {
	worker := NewSimpleWorker(1)

	f, err := openFileOrStdin(fpath)
	if err != nil {
		worker.AddError(err)
		return worker
	}

	var recentIDs []storj.NodeID
	recentIDsPos := 0
	if recentSkipSize > 0 {
		recentIDs = make([]storj.NodeID, recentSkipSize)
	}

	lineF := bufio.NewReader(f)
	go func() {
		defer func() {
			f.Close()
			worker.Done()
		}()
	lines_loop:
		for {
			line, err := lineF.ReadString('\n')
			if err == io.EOF {
				if line == "" {
					break
				}
			} else if err != nil {
				worker.AddError(err)
				return
			}
			node := &pb.Node{}
			if err := jsonpb.UnmarshalString(line, node); err != nil {
				worker.AddError(err)
				return
			}
			if recentIDs != nil {
				for _, recentID := range recentIDs {
					if recentID == node.Id {
						logInfo("KAD-IMPORT", "skipping recent %s", node.Id)
						continue lines_loop
					}
				}
				recentIDs[recentIDsPos] = node.Id
				recentIDsPos = (recentIDsPos + 1) % len(recentIDs)
			}
			kadDataChan <- node
		}
		if infinite {
			worker.AddError(merry.New("expcted to import KAD data infinitely, but file has ended"))
		}
	}()

	return worker
}
