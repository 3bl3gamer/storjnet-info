package nodes

import (
	"testing"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

func Test_dedupLimits(t *testing.T) {
	limit := func(idByte byte, addr string) *pb.AddressedOrderLimit {
		id := storj.NodeID{}
		id[0] = idByte
		return &pb.AddressedOrderLimit{
			Limit:              &pb.OrderLimit{StorageNodeId: id},
			StorageNodeAddress: &pb.NodeAddress{Address: addr},
		}
	}

	// several draws in a row return overlapping node sets
	limits := []*pb.AddressedOrderLimit{
		limit(1, "a:1"), limit(2, "b:1"),
		limit(2, "b:1"), limit(3, "c:1"),
		limit(1, "a:1"),
	}

	unique := dedupLimits(limits)
	if len(unique) != 3 {
		t.Fatalf("expected 3 unique limits, got %d", len(unique))
	}
	for i, addr := range []string{"a:1", "b:1", "c:1"} {
		if unique[i].StorageNodeAddress.Address != addr {
			t.Errorf("limit %d: expected %s, got %s", i, addr, unique[i].StorageNodeAddress.Address)
		}
	}

	if got := dedupLimits(nil); len(got) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(got))
	}
}
