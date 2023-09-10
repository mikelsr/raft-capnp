package main

import (
	"context"
	"errors"
	"time"

	"github.com/mikelsr/raft-capnp/proto/api"
	"github.com/mikelsr/raft-capnp/raft"
)

// Users need to define a node retrieval function. In the example we'll use
// a global map that all goroutines can access. It'll be read-only after
// the initial steps, so no thread-safety measures need to be taken.
var nodes map[uint64]api.Raft

func retrieve(ctx context.Context, u uint64) (api.Raft, error) {
	n, ok := nodes[u]
	if !ok {
		return api.Raft{}, errors.New("node not found")
	}
	return n, nil
}

// Create three Raft nodes.
// Register their capabilities on the aforementioned map.
// Start one of them (n1) and wait until it becomes the leader of the Raft.
// Start n2 and n3.
// Join n1 from n2 and n3.
// The log will show what is happenning in the Raft,
// feel free to play around and add or remove items and nodes.
func main() {
	nodes = make(map[uint64]api.Raft)

	ctx, cancel := context.WithCancel(context.Background())

	n1 := raft.New().
		WithRaftConfig(raft.DefaultConfig()).
		WithRaftStore(raft.DefaultRaftStore).
		WithStorage(raft.DefaultStorage()).
		WithOnNewValue(raft.NilOnNewValue).
		WithRaftNodeRetrieval(retrieve).
		WithLogger(raft.DefaultLogger(true))

	n2 := raft.New().
		WithRaftConfig(raft.DefaultConfig()).
		WithRaftStore(raft.DefaultRaftStore).
		WithStorage(raft.DefaultStorage()).
		WithOnNewValue(raft.NilOnNewValue).
		WithRaftNodeRetrieval(retrieve).
		WithLogger(raft.DefaultLogger(true))

	n3 := raft.New().
		WithRaftConfig(raft.DefaultConfig()).
		WithRaftStore(raft.DefaultRaftStore).
		WithStorage(raft.DefaultStorage()).
		WithOnNewValue(raft.NilOnNewValue).
		WithRaftNodeRetrieval(retrieve).
		WithLogger(raft.DefaultLogger(true))

	c1, c2, c3 := n1.Cap(), n2.Cap(), n3.Cap()
	nodes[n1.ID] = c1
	nodes[n2.ID] = c2
	nodes[n3.ID] = c3

	// We need to init n1 now so we can check Raft.Status()
	// even before n1.Start calls Init implicitly.
	// TODO mikel: find a cleaner way
	n1.Init()

	go n1.Start(ctx)

	// Wait for N1 to become the leader.
	for n1.Raft.Status().Lead != n1.ID {
		time.Sleep(10 * time.Millisecond)
	}

	go n2.Start(ctx)
	go n3.Start(ctx)

	// n2 joins n1.
	c1.Add(ctx, func(r api.Raft_add_Params) error {
		return r.SetNode(c2.AddRef())
	})

	// n3 joins n1.
	c1.Add(ctx, func(r api.Raft_add_Params) error {
		return r.SetNode(c3.AddRef())
	})

	// Watch the logs.
	time.Sleep(5 * time.Second)
	cancel()
}
