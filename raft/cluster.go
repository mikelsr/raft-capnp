package raft

import (
	"context"
	"sync"

	"github.com/mikelsr/raft-capnp/proto/api"
)

// Cluster represents a set of active
// raft members
type Cluster struct {
	lock sync.RWMutex
	// TODO replace with sync.Map
	peers map[uint64]api.Raft
}

// NewCluster creates a new cluster neighbors
// list for a raft member
func NewCluster() *Cluster {
	return &Cluster{
		peers: make(map[uint64]api.Raft),
	}
}

// Peers returns the list of peers in the cluster
func (c *Cluster) Peers() map[uint64]api.Raft {
	peers := make(map[uint64]api.Raft)
	c.lock.RLock()
	for k, v := range c.peers {
		peers[k] = v
	}
	c.lock.RUnlock()
	return peers
}

// AddPeer adds a node to our neighbors
func (c *Cluster) AddPeer(ctx context.Context, peer api.Raft) {
	c.lock.Lock()
	id, _ := rpcGetId(ctx, peer)
	c.peers[id] = peer
	c.lock.Unlock()
}

// RemovePeer removes a node from our neighbors
func (c *Cluster) RemovePeer(id uint64) {
	c.lock.Lock()
	delete(c.peers, id)
	c.lock.Unlock()
}
