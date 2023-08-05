package raft

import (
	"context"

	"github.com/mikelsr/raft-capnp/proto/api"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

// Node implements api.Raft_Server.
type Node struct {
	// raft specifics
	raft  raft.Node
	queue []raftpb.Message

	Id uint64
	Cluster
}

func (n *Node) Join(ctx context.Context, call api.Raft_join) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	nodes, err := n.join(ctx, n.Info())
	if err != nil {
		res.SetError(err.Error())
		return err
	}
	caps, err := res.NewNodes(int32(len(nodes)))
	if err != nil {
		res.SetError(err.Error())
		return err
	}
	for i, n := range nodes {
		caps.Set(i, n.Cap())
	}
	if err = res.SetNodes(caps); err != nil {
		res.SetError(err.Error())
		return err
	}
	return nil
}

func (n *Node) Leave(ctx context.Context, call api.Raft_leave) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	if err = n.leave(ctx, n.Info()); err != nil {
		res.SetError(err.Error())
		return err
	}
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

func (n *Node) Info() Info {
	return Info{
		ID:   n.Id,
		Chan: nil, // TODO create a channel cap
	}
}

// call with n.Info()
func (n *Node) join(ctx context.Context, info Info) ([]Info, error) {

	cc := raftpb.ConfChange{
		ID:     info.ID,
		Type:   raftpb.ConfChangeAddNode,
		NodeID: info.ID,
		// TODO how can we pass our channel to the remote peers?
		// Context: marshaledCap,
	}

	if err := n.raft.ProposeConfChange(ctx, cc); err != nil {
		return nil, err
	}

	// TODO is the map necessary?
	peers := n.Cluster.Peers()
	nodes := make([]Info, len(peers))
	i := 0
	for _, node := range peers {
		nodes[i] = node
		i++
	}

	return nodes, nil
}

// call with n.Info()
func (n *Node) leave(ctx context.Context, info Info) error {
	cc := raftpb.ConfChange{
		ID:     info.ID,
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: info.ID,
	}
	return n.raft.ProposeConfChange(ctx, cc)
}
