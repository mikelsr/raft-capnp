package raft

import (
	"context"
	"sync"

	"github.com/mikelsr/raft-capnp/proto/api"
)

// View represents a set of active
// raft members
type View struct {
	peers *sync.Map
}

// NewView creates a new cluster neighbors
// list for a raft member
func NewView() *View {
	return &View{
		peers: &sync.Map{},
	}
}

// Peers returns the list of peers in the cluster
func (c *View) Peers() map[uint64]api.Raft {
	peers := make(map[uint64]api.Raft)
	c.peers.Range(func(key, value any) bool {
		peers[key.(uint64)] = value.(api.Raft).AddRef()
		return true
	})
	return peers
}

// AddPeer adds a node to our neighbors
func (c *View) AddPeer(ctx context.Context, peer api.Raft) {
	id, _ := rpcGetId(ctx, peer.AddRef())
	c.peers.Store(id, peer.AddRef())
}

// PopPeer removes a node from our neighbors
func (c *View) PopPeer(id uint64) api.Raft {
	peer, ok := c.peers.LoadAndDelete(id)
	if !ok {
		return api.Raft{}
	}
	return peer.(api.Raft).AddRef()
}
