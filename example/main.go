package main

import (
	"context"
	"errors"
	"time"

	"github.com/mikelsr/raft-capnp/raft"
)

func main() {
	n := raft.New().
		WithRaftConfig(raft.DefaultConfig()).
		WithRaftStore(raft.DefaultRaftStore).
		WithStorage(raft.DefaultStorage()).
		WithRaftNodeRetrieval(raft.NilRaftNodeRetrieval).
		WithOnNewValue(raft.NilOnNewValue)
	defer func() {
		time.Sleep(10 * time.Millisecond)
		n.Stop(errors.New("bye"))
	}()

	n.Start(context.Background())
}
