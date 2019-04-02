package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	// Addr is the address of peer from command flags
	Addr = flag.String("address", "127.0.0.1:7778", "address of peer to inspect")

	// ErrInspectorDial throws when there are errors dialing the inspector server
	ErrInspectorDial = errs.Class("error dialing inspector server:")

	// ErrRequest is for gRPC request errors after dialing
	ErrRequest = errs.Class("error processing request:")

	// ErrArgs throws when there are errors with CLI args
	ErrArgs = errs.Class("error with CLI args:")

	// Commander CLI
	rootCmd = &cobra.Command{
		Use:   "inspector",
		Short: "CLI for interacting with Storj Kademlia network",
	}
	kadCmd = &cobra.Command{
		Use:   "kad",
		Short: "commands for kademlia/overlay cache",
	}
	nodeInfoCmd = &cobra.Command{
		Use:   "node-info <node_id>",
		Short: "get node info directly from node",
		Args:  cobra.MinimumNArgs(1),
		RunE:  NodeInfo,
	}
	dumpNodesCmd = &cobra.Command{
		Use:   "dump-nodes",
		Short: "dump all nodes in the routing table",
		RunE:  DumpNodes,
	}
	startRecordingCmd = &cobra.Command{
		Use:   "start-recording",
		Short: "",
		RunE:  StartRecording,
	}
	addNodesByIDCmd = &cobra.Command{
		Use:   "add-nodes-by-id",
		Short: "",
		Args:  cobra.MinimumNArgs(1),
		RunE:  AddNodesById,
	}
	addNodesFromFile = &cobra.Command{
		Use:   "add-nodes-from-file",
		Short: "",
		Args:  cobra.MinimumNArgs(1),
		RunE:  AddNodesFromFile,
	}
)

// Inspector gives access to kademlia, overlay cache
type Inspector struct {
	kadclient pb.KadInspectorClient
}

// NewInspector creates a new gRPC inspector client for access to kad,
// overlay cache
func NewInspector(address string) (*Inspector, error) {
	ctx := context.Background()

	conn, err := transport.DialAddressInsecure(ctx, address)
	if err != nil {
		return &Inspector{}, ErrInspectorDial.Wrap(err)
	}

	return &Inspector{
		kadclient: pb.NewKadInspectorClient(conn),
	}, nil
}

// NodeInfo get node info directly from the node with provided Node ID
func NodeInfo(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	// first lookup the node to get its address
	n, err := i.kadclient.LookupNode(context.Background(), &pb.LookupNodeRequest{
		Id: args[0],
	})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Println(prettyPrint(n))

	// now ask the node directly for its node info
	info, err := i.kadclient.NodeInfo(context.Background(), &pb.NodeInfoRequest{
		Id:      n.GetNode().Id,
		Address: n.GetNode().GetAddress(),
	})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Println(prettyPrint(info))

	return nil
}

// DumpNodes outputs a json list of every node in every bucket in the satellite
func DumpNodes(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	nodes, err := i.kadclient.FindNear(context.Background(), &pb.FindNearRequest{
		Start: storj.NodeID{},
		Limit: 100000,
	})
	if err != nil {
		return err
	}

	fmt.Println(prettyPrint(nodes))

	return nil
}

func prettyPrint(unformatted proto.Message) string {
	m := jsonpb.Marshaler{Indent: "  ", EmitDefaults: true}
	formatted, err := m.MarshalToString(unformatted)
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
	return formatted
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

type DbTx interface {
	QueryOne(interface{}, interface{}, ...interface{}) (orm.Result, error)
	Exec(interface{}, ...interface{}) (orm.Result, error)
	RunInTransaction(func(*pg.Tx) error) error
}

func saveNode(db DbTx, node *pb.Node) (bool, error) {
	m := jsonpb.Marshaler{Indent: "", EmitDefaults: true}
	str, err := m.MarshalToString(node)
	if err != nil {
		return false, err
	}
	var xmax string
	_, err = db.QueryOne(&xmax, `
		INSERT INTO storj3_nodes (id, params, updated_at) VALUES (?, ?::jsonb, NOW())
		ON CONFLICT (id) DO UPDATE SET params = EXCLUDED.params, updated_at = NOW()
		RETURNING xmax`,
		node.Id, str)
	if err != nil {
		return false, err
	}
	return xmax == "0", nil
}

func incWrongCounter(db DbTx, nodeStrID string) error {
	id, err := storj.NodeIDFromString(nodeStrID)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO storj3_wrong_ids (id, counter) VALUES (?, 1)
		ON CONFLICT (id) DO UPDATE SET counter = storj3_wrong_ids.counter + 1`,
		id)
	return err
}

func saveNodes(db DbTx, nodes []*pb.Node) (int, error) {
	createdCount := 0
	err := db.RunInTransaction(func(tx *pg.Tx) error {
		for _, node := range nodes {
			created, err := saveNode(tx, node)
			if err != nil {
				return err
			}
			if created {
				createdCount++
			}
		}
		return nil
	})
	if err != nil {
		return createdCount, err
	}
	return createdCount, nil
}

func fetchNode(db DbTx, ins *Inspector, nodeID string) (*pb.Node, error) {
	resp, err := ins.kadclient.LookupNode(context.Background(), &pb.LookupNodeRequest{
		Id: nodeID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			if st.Message() == "node not found" {
				fmt.Printf("skipping %s: not found\n", nodeID)
				if err := incWrongCounter(db, nodeID); err != nil {
					panic(err)
				}
			} else {
				fmt.Printf("skipping %s: strange message: %s\n", nodeID, st.Message())
			}
		} else {
			fmt.Printf("skipping %s: strange reason: %s\n", nodeID, ErrRequest.Wrap(err))
		}
		return nil, err
	}
	return resp.GetNode(), nil
}

func startNodesFetching(db DbTx, ins *Inspector, nodeStrIDsChan chan string) chan *pb.Node {
	nodesChan := make(chan *pb.Node)
	n := 8

	wg := sync.WaitGroup{}
	for i := 0; i < n; i++ {
		go func() {
			wg.Add(1)
			for strID := range nodeStrIDsChan {
				node, err := fetchNode(db, ins, strID)
				if err == nil {
					nodesChan <- node
				} else {
					nodesChan <- nil
				}
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(nodesChan)
	}()
	return nodesChan
}

type NodesSaveResult struct {
	nodes        []*pb.Node
	totalCount   int
	createdCount int
	err          error
}

func startNodesSaving(db DbTx, nodesChan chan *pb.Node) chan NodesSaveResult {
	resultsChan := make(chan NodesSaveResult)
	groupSize := 16
	go func() {
		totalCount := 0
		nodes := make([]*pb.Node, 0)
	loop:
		for {
			select {
			case node, ok := <-nodesChan:
				if !ok {
					break loop
				}
				totalCount++
				if node != nil {
					nodes = append(nodes, node)
				}
				if totalCount < groupSize {
					continue loop
				}
			case <-time.After(5 * time.Second):
			}

			if len(nodes) > 0 {
				cc, err := saveNodes(db, nodes)
				resultsChan <- NodesSaveResult{append(nodes[:0:0], nodes...), totalCount, cc, err}
				if err != nil {
					break
				}
				nodes = nodes[:0]
				totalCount = 0
			}
		}
		close(resultsChan)
	}()
	return resultsChan
}

func fetchAndSaveNodes(db DbTx, ins *Inspector, nodeIDs []string) (int, int, error) {
	totalCount := 0
	savedCount := 0
	createdCount := 0
	startStamp := time.Now().Unix()

	nodeStrIDsChan := make(chan string)
	go func() {
		for _, id := range nodeIDs {
			nodeStrIDsChan <- id
		}
		close(nodeStrIDsChan)
	}()
	nodesChan := startNodesFetching(db, ins, nodeStrIDsChan)
	resultsChan := startNodesSaving(db, nodesChan)
	for res := range resultsChan {
		if res.err != nil {
			return 0, 0, res.err
		}
		totalCount += res.totalCount
		savedCount += len(res.nodes)
		createdCount += res.createdCount
		speed := float64(totalCount) / float64(time.Now().Unix()-startStamp)
		fmt.Printf("total: %d/%d, speed: %0.2f n/s, chunk: %d/%d, new: %d\n",
			totalCount, len(nodeIDs), speed, len(res.nodes), res.totalCount, res.createdCount)
	}

	return savedCount, createdCount, nil
}

func StartRecording(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	db := makePGConnection()

	nodes, err := i.kadclient.FindNear(context.Background(), &pb.FindNearRequest{
		Start: storj.NodeID{},
		Limit: 100000,
	})
	if err != nil {
		return err
	}

	createdCount, err := saveNodes(db, nodes.GetNodes())
	if err != nil {
		return err
	}
	fmt.Printf("nodes: %d total, %d new\n", len(nodes.GetNodes()), createdCount)

	return nil
}

func AddNodesById(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	db := makePGConnection()

	fmt.Printf("adding %d node(s)\n", len(args))
	savedCount, createdCount, err := fetchAndSaveNodes(db, i, args)
	if err != nil {
		return err
	}
	fmt.Printf("nodes: %d total, %d saved, %d new\n", len(args), savedCount, createdCount)
	return nil
}

func AddNodesFromFile(cmd *cobra.Command, args []string) (err error) {
	ins, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	db := makePGConnection()

	fpath := args[0]
	var f *os.File

	if fpath == "-" {
		f = os.Stdin
	} else {
		f, err = os.Open("-")
		if err != nil {
			return err
		}
	}
	lineF := bufio.NewReader(f)

	nodeStrIDsChan := make(chan string, 64)
	go func() {
		lastIDs := make([]string, 0)
		for i := 0; ; i++ {
			line, err := lineF.ReadString('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
			nodeID := line[:len(line)-1]
			if f == os.Stdin && len(nodeStrIDsChan) == cap(nodeStrIDsChan) {
				println("buffer is full, skipping")
				continue
			}

			found := false
			for _, lastID := range lastIDs {
				if lastID == nodeID {
					println("node ID is recent, skipping")
					found = true
					break
				}
			}
			if !found {
				nodeStrIDsChan <- nodeID
				if len(lastIDs) < 128 {
					lastIDs = append(lastIDs, nodeID)
				} else {
					for i := 0; i < len(lastIDs)-1; i++ {
						lastIDs[i] = lastIDs[i+1]
					}
					lastIDs[len(lastIDs)-1] = nodeID
				}
			}
			if i%50 == 0 {
				fmt.Printf("file buf fill: %d/%d\n", len(nodeStrIDsChan), cap(nodeStrIDsChan))
			}
		}
		close(nodeStrIDsChan)
	}()
	nodesChan := startNodesFetching(db, ins, nodeStrIDsChan)
	resultsChan := startNodesSaving(db, nodesChan)
	totalCount := 0
	createdCount := 0
	startStamp := time.Now().Unix()
	for res := range resultsChan {
		if res.err != nil {
			return res.err
		}
		totalCount += res.totalCount
		createdCount += res.createdCount
		speed := float64(totalCount) / float64(time.Now().Unix()-startStamp)
		fmt.Printf("total: %d, new: %d, speed: %.2f n/s | chunk: %d/%d, new: %d\n",
			totalCount, createdCount, speed, len(res.nodes), res.totalCount, res.createdCount)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(kadCmd)
	rootCmd.AddCommand(startRecordingCmd)
	rootCmd.AddCommand(addNodesByIDCmd)
	rootCmd.AddCommand(addNodesFromFile)

	kadCmd.AddCommand(nodeInfoCmd)
	kadCmd.AddCommand(dumpNodesCmd)

	flag.Parse()
}

func main() {
	process.Exec(rootCmd)
}
