package raft

import (
	"fmt"
	"log"
	"math/rand"
	"os"

	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

var DefaultLogger = &raft.DefaultLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)}

func DefaultConfig() *raft.Config {
	return &raft.Config{
		HeartbeatTick:   HeartbeatTick,
		ElectionTick:    ElectionTick,
		MaxSizePerMsg:   MaxSizePerMsg,
		MaxInflightMsgs: MaxInflightMsgs,
	}
}

func DefaultID() uint64 {
	return rand.Uint64()
}

func DefaultStorage() raft.Storage {
	return raft.NewMemoryStorage()
}

func DefaultRaftStore(storage raft.Storage, hardState raftpb.HardState, entries []raftpb.Entry, snapshot raftpb.Snapshot) error {
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
