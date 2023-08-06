package main

import (
	"context"
	"errors"
	"time"

	"github.com/mikelsr/raft-capnp/proto/api"
	"github.com/mikelsr/raft-capnp/raft"
)

func main() {
	n := raft.New().
		WithRaftConfig(raft.DefaultConfig()).
		WithRaftStore(raft.DefaultRaftStore).
		WithStorage(raft.DefaultStorage()).
		WithRaftNodeRetrieval(retrieve).
		WithOnNewValue(raft.NilOnNewValue)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(2 * time.Second)
		cancel()
		// n.Stop(errors.New("bye"))
	}()

	n.Start(ctx)
}

func retrieve(ctx context.Context, id uint64) (api.Raft, error) {
	time.Sleep(10 * time.Second)
	return api.Raft{}, errors.New("unimplemented")
}
