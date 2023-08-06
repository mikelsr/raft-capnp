package raft

import (
	"context"
	"errors"

	"github.com/mikelsr/raft-capnp/proto/api"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

// RaftStore performs an store on the given storage. Storage will be of the type
// supplied to the node.
type RaftStore func(storage raft.Storage, hardState raftpb.HardState, entries []raftpb.Entry, snapshot raftpb.Snapshot) error

// NilRaftStore defines a null behaviour for RaftStore.
// WARNING: IT WILL MAKE Node FAIL.
func NilRaftStore(storage raft.Storage, hardState raftpb.HardState, entries []raftpb.Entry, snapshot raftpb.Snapshot) error {
	return nil
}

// RaftNodeRetrieval returns the raft node capability corresponding to a
// node ID. It MUST be implemented and supplied to Node.
type RaftNodeRetrieval func(context.Context, uint64) (api.Raft, error)

// NilRaftNodeRetrieval defines a null behaviour for RaftNodeRetrieval.
// WARNING: IT WILL MAKE Node FAIL.
func NilRaftNodeRetrieval(ctx context.Context, id uint64) (api.Raft, error) {
	return api.Raft{}, errors.New("unimplemented")
}

// OnNewValue will be executed each time Raft node receives a new value.
// Optional.
type OnNewValue func(Item) error

// NilOnNewValue defines a null behaviour for OnNewVaue.
func NilOnNewValue(Item) error {
	return nil
}
