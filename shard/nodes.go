package shard

import "fmt"

type Node struct {
	NodeID  uint64
	ShardID uint64
	IpAddr  string
}

func (n *Node) PrintNode() {
	v := []interface{}{
		n.NodeID,
		n.ShardID,
		n.IpAddr,
	}
	fmt.Printf("%v\n", v)
}
