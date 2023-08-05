package raft

import (
	"context"

	"github.com/mikelsr/raft-capnp/proto/api"
)

// Node implements api.Raft_Server.
type Node struct {
	Id uint64
}

func (n *Node) Join(ctx context.Context, call api.Raft_join) error {
	return nil
}

func (n *Node) Leave(ctx context.Context, call api.Raft_leave) error {
	return nil
}

func (n *Node) Send(ctx context.Context, call api.Raft_send) error {
	return nil
}

func (n *Node) Put(ctx context.Context, call api.Raft_put) error {
	return nil
}

func (n *Node) List(ctx context.Context, call api.Raft_list) error {
	return nil
}

func (n *Node) Members(ctx context.Context, call api.Raft_members) error {
	return nil
}
