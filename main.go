package main

import (
	"context"
	"flag"
	"fmt"
	"os"

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
	addNodeByIdCmd = &cobra.Command{
		Use:   "add-node-by-id",
		Short: "",
		Args:  cobra.MinimumNArgs(1),
		RunE:  AddNodeById,
	}
)

// Inspector gives access to kademlia, overlay cache
type Inspector struct {
	kadclient     pb.KadInspectorClient
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

func saveNodes(db *pg.DB, nodes []*pb.Node) (int, error) {
	createdCount := 0
	err := db.RunInTransaction(func (tx *pg.Tx) error {
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

func fetchAndSaveNodes(db DbTx, ins *Inspector, nodeIDs []string) ([]*pb.Node, int, error) {
	createdCount := 0
	nodes := make([]*pb.Node, 0, len(nodeIDs))
	for i, id := range nodeIDs {
		resp, err := ins.kadclient.LookupNode(context.Background(), &pb.LookupNodeRequest{
			Id: id,
		})
		if err != nil {
			if st, ok := status.FromError(err); ok {
				if st.Message() == "node not found" {
					fmt.Printf("skipping %s: not found\n", id)
					if err := incWrongCounter(db, id);  err != nil {
						return nil, createdCount, err
					}
				} else {
					fmt.Printf("skipping %s: strange message: %s\n", id, st.Message())
				}
			} else {
				fmt.Printf("skipping %s: strange reason: %s\n", id, ErrRequest.Wrap(err))
			}
			continue
		}
		node := resp.GetNode()
		nodes = append(nodes, node)
		created, err := saveNode(db, node)
		if err != nil {
			return nodes, createdCount, err
		}
		cChar := '.'
		if created {
			createdCount++
			cChar = '+'
		}
		fmt.Printf("added %d/%d %c %s\n", i+1, len(nodeIDs), cChar, node.Id)
	}
	return nodes, createdCount, nil
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

func AddNodeById(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	db := makePGConnection()

	fmt.Printf("adding %d node(s)\n", len(args))
	nodes, createdCount, err := fetchAndSaveNodes(db, i, args)
	if err != nil {
		return err
	}
	fmt.Printf("nodes: %d total, %d found, %d new\n", len(args), len(nodes), createdCount)
	return nil
}

func init() {
	rootCmd.AddCommand(kadCmd)
	rootCmd.AddCommand(startRecordingCmd)
	rootCmd.AddCommand(addNodeByIdCmd)

	kadCmd.AddCommand(nodeInfoCmd)
	kadCmd.AddCommand(dumpNodesCmd)

	flag.Parse()
}

func main() {
	process.Exec(rootCmd)
}
