package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/spf13/cobra"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	rootCmd = &cobra.Command{
		Use:   "storj3stat",
		Short: "Tool for gathering storj network stats",
	}
	importCmd = &cobra.Command{
		Use:   "import",
		Short: "import nodes data",
	}
	importIDsCmd = &cobra.Command{
		Use:   "ids",
		Short: "import raw node IDs",
		Args:  cobra.MinimumNArgs(1),
		RunE:  ImportNodeIDs,
	}
	importKadDataCmd = &cobra.Command{
		Use:   "kad",
		Short: "import nodes kad data",
		Args:  cobra.MinimumNArgs(1),
		RunE:  ImportNodesKadData,
	}
)

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.AddCommand(importIDsCmd)
	importCmd.AddCommand(importKadDataCmd)
}

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

func ImportNodeIDs(cmd *cobra.Command, args []string) (err error) {
	f, err := openFileOrStdin(args[0])
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

func ImportNodesKadData(cmd *cobra.Command, args []string) (err error) {
	f, err := openFileOrStdin(args[0])
	if err != nil {
		return merry.Wrap(err)
	}
	defer f.Close()

	db := makePGConnection()
	kadDataChan := make(chan interface{}, 16)
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
			node := &pb.Node{}
			if err := jsonpb.Unmarshal(strings.NewReader(line), node); err != nil {
				errChan <- merry.Wrap(err)
				break
			}
			kadDataChan <- node
		}
		close(kadDataChan)
	}()

	count := 0
	countNew := 0
	err = saveChunked(db, 100, kadDataChan, func(tx *pg.Tx, items []interface{}) error {
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
			count++
			if xmax == "0" {
				countNew++
			}
		}
		log.Printf("imported %d kad nodes, %d new", count, countNew)
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

	log.Printf("done, imported %d kad nodes, %d new", count, countNew)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Print(merry.Details(err))
	}
}
