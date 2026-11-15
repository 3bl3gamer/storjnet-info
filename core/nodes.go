package core

import (
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v10"
	"storj.io/common/storj"
)

type BriefNode struct {
	RawID   []byte       `json:"-"`
	ID      storj.NodeID `json:"id"`
	Address string       `json:"address"`
}

// SatNode is a satellite pinged for the main page.
type SatNode struct {
	BriefNode
	Group string `json:"group"` //stats group, see SatelliteGroup
}

type Node struct {
	BriefNode
	PingMode      string    `json:"pingMode"`
	LastPingedAt  time.Time `json:"lastPingedAt"`
	LastPing      int64     `json:"lastPing"`
	LastPingWasOk bool      `json:"lastPingWasOk"`
	LastUpAt      time.Time `json:"lastUpAt"`
	CreatedAt     time.Time `json:"-"`
}

type UserNode struct {
	Node
	UserID int64
}

func ConvertBriefNodeIDs(nodes []*BriefNode) error {
	var err error
	for _, node := range nodes {
		node.ID, err = storj.NodeIDFromBytes(node.RawID)
		if err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
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
		INSERT INTO user_nodes (node_id, user_id, address, ping_mode, details_updated_at) VALUES (?, ?, ?, ?, now())
		ON CONFLICT (node_id, user_id) DO UPDATE SET
			address = EXCLUDED.address,
			ping_mode = EXCLUDED.ping_mode,
			details_updated_at = now()`,
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
	_, err := db.Query(&nodes, `
		SELECT node_id AS raw_id, address, ping_mode, last_pinged_at, last_ping, last_ping_was_ok, last_up_at
		FROM user_nodes WHERE user_id = ?`, user.ID)
	if err != nil {
		return nil, merry.Wrap(err)
	}
	if err := ConvertNodeIDs(nodes); err != nil {
		return nil, merry.Wrap(err)
	}
	return nodes, nil
}

func LoadSatNodes(db *pg.DB, startDate, endDate time.Time) ([]*SatNode, error) {
	nodes := make([]*SatNode, 0)
	_, err := db.Query(&nodes, `
		SELECT node_id AS raw_id, address FROM user_nodes
		WHERE user_id IN (SELECT id FROM users WHERE username IN (?))
		  AND EXISTS (
			SELECT 1 FROM user_nodes_history
			WHERE node_id = user_nodes.node_id
			  AND user_id = user_nodes.user_id
			  AND date BETWEEN ? AND ?
		  )`,
		pg.In(SatUsernames()), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, merry.Wrap(err)
	}
	for _, node := range nodes {
		id, err := storj.NodeIDFromBytes(node.RawID)
		if err != nil {
			return nil, merry.Wrap(err)
		}
		node.ID = id
		node.Group = SatelliteGroup(node.Address)
	}
	return nodes, nil
}
