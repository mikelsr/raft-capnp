package main

import "github.com/mikelsr/raft-capnp/raft"

func main() {
	_ = raft.New().
		WithRaftConfig(raft.DefaultConfig()).
		WithRaftStore(raft.DefaultRaftStore).
		WithStorage(raft.DefaultStorage()).
		WithRaftNodeRetrieval(raft.NilRaftNodeRetrieval).
		WithOnNewValue(raft.NilOnNewValue)
}
