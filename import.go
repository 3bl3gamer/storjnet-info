package main

import (
	"bufio"
	"io"
	"log"
	"strings"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
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
			res, err := db.Exec(`INSERT INTO storj3_nodes (id) VALUES (?) ON CONFLICT (id) DO NOTHING RETURNING xmax`, id)
			if err != nil {
				return merry.Wrap(err)
			}
			count++
			if res.RowsAffected() > 0 {
				countNew++
			}
		}
		log.Printf("imported %d IDs, %d new", count, countNew)
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

	log.Printf("done")
	return nil
}

func ImportNodesKadData(fpath string) (err error) {
	f, err := openFileOrStdin(fpath)
	if err != nil {
		return merry.Wrap(err)
	}
	defer f.Close()

	db := makePGConnection()
	saver := StartNodesKadDataSaving(db)
	defer saver.Stop()

	lineF := bufio.NewReader(f)
	for {
		line, err := lineF.ReadString('\n')
		if err == io.EOF {
			if line == "" {
				break
			}
		} else if err != nil {
			return merry.Wrap(err)
		}
		node := &pb.Node{}
		if err := jsonpb.Unmarshal(strings.NewReader(line), node); err != nil {
			return merry.Wrap(err)
		}
		saver.Add(node)
	}

	if err := saver.StopAndWait(); err != nil {
		return merry.Wrap(err)
	}
	return nil
}
