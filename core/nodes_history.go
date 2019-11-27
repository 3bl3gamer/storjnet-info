package core

import (
	"time"

	"storj.io/storj/pkg/storj"
)

type UserNodeHistory struct {
	tableName struct{}     `pg:"user_nodes_history"`
	RawNodeID []byte       `json:"-"`
	NodeID    storj.NodeID `json:"nodeId"`
	UserID    int64        `json:"userId"`
	Date      time.Time    `json:"date"`
	Pings     []uint16     `json:"pings" pg:",array"`
}
