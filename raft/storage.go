package raft

import (
	"fmt"

	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

func DefaultStorage() raft.Storage {
	return raft.NewMemoryStorage()
}

func DefaultStoreFunc(storage raft.Storage, hardState raftpb.HardState, entries []raftpb.Entry, snapshot raftpb.Snapshot) error {
	s, ok := storage.(*raft.MemoryStorage)
	if !ok {
		return fmt.Errorf("failed to cast %v to raft.MemoryStorage", s)
	}
	s.Append(entries)

	if !raft.IsEmptyHardState(hardState) {
		s.SetHardState(hardState)
	}

	if !raft.IsEmptySnap(snapshot) {
		s.ApplySnapshot(snapshot)
	}
	return nil
}
