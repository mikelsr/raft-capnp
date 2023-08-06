package raft

import "go.etcd.io/raft/v3/raftpb"

// Storage expands raft.Storage to include the methods found in raft.MemoryStorage.
type Storage interface {
	Append(entries []raftpb.Entry) error
	ApplySnapshot(snap raftpb.Snapshot) error
	Compact(compactIndex uint64) error
	CreateSnapshot(i uint64, cs *raftpb.ConfState, data []byte) (raftpb.Snapshot, error)
	Entries(lo uint64, hi uint64, maxSize uint64) ([]raftpb.Entry, error)
	FirstIndex() (uint64, error)
	InitialState() (raftpb.HardState, raftpb.ConfState, error)
	LastIndex() (uint64, error)
	SetHardState(st raftpb.HardState) error
	Snapshot() (raftpb.Snapshot, error)
	Term(i uint64) (uint64, error)
}
