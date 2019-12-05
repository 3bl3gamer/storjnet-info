package core

import (
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"storj.io/storj/pkg/storj"
)

type Node struct {
	RawID        []byte       `json:"-"`
	ID           storj.NodeID `json:"id"`
	Address      string       `json:"address"`
	PingMode     string       `json:"pingMode"`
	LastPingedAt time.Time    `json:"lastPingedAt"`
	LastPing     int64        `json:"lastPing"`
	LastUpAt     time.Time    `json:"lastUpAt"`
	CreatedAt    time.Time    `json:"-"`
}

type UserNode struct {
	Node
	UserID int64
}

func ConvertNodeIDs(nodes []*Node) error {
	var err error
	for _, node := range nodes {
		node.ID, err = storj.NodeIDFromBytes(node.RawID)
		if err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}

func ConvertUserNodeIDs(nodes []*UserNode) error {
	var err error
	for _, node := range nodes {
		node.ID, err = storj.NodeIDFromBytes(node.RawID)
		if err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}

func SetUserNode(db *pg.DB, user *User, node *Node) error {
	_, err := db.Exec(`
		INSERT INTO user_nodes (node_id, user_id, address, ping_mode) VALUES (?, ?, ?, ?)
		ON CONFLICT (node_id, user_id) DO UPDATE SET address = EXCLUDED.address, ping_mode = EXCLUDED.ping_mode`,
		node.ID, user.ID, node.Address, node.PingMode)
	return merry.Wrap(err)
}

func DelUserNode(db *pg.DB, user *User, nodeID storj.NodeID) error {
	_, err := db.Exec(`
		DELETE FROM user_nodes WHERE node_id = ? AND user_id = ?`,
		nodeID, user.ID)
	return merry.Wrap(err)
}

func LoadUserNodes(db *pg.DB, user *User) ([]*Node, error) {
	nodes := make([]*Node, 0)
	_, err := db.Query(&nodes, "SELECT node_id AS raw_id, address, ping_mode FROM user_nodes WHERE user_id = ?", user.ID)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	if err := ConvertNodeIDs(nodes); err != nil {
		return nil, merry.Wrap(err)
	}
	return nodes, nil
}

func LoadSatNodes(db *pg.DB) ([]*Node, error) {
	nodes := make([]*Node, 0)
	_, err := db.Query(&nodes, `
		SELECT node_id AS raw_id, address FROM user_nodes
		WHERE user_id = (SELECT id FROM users WHERE email = 'satellites@mail.com')`)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	if err := ConvertNodeIDs(nodes); err != nil {
		return nil, merry.Wrap(err)
	}
	return nodes, nil
}
