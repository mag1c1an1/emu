package shard

import "fmt"

type Node struct {
	NodeId  uint64
	ShardId uint64
	IpAddr  string
}

func (n *Node) PrintNode() {
	v := []interface{}{
		n.NodeId,
		n.ShardId,
		n.IpAddr,
	}
	fmt.Printf("%v\n", v)
}
