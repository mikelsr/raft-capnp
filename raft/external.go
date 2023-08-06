package raft

import (
	"errors"

	"github.com/mikelsr/raft-capnp/proto/api"
)

// RaftNodeRetrieval returns the raft node capability corresponding to a
// node ID. It MUST be implemented and supplied to Node.
type RaftNodeRetrieval func(uint64) (api.Raft, error)

// NilRaftNodeRetrieval defines a null behaviour for RaftNodeRetrieval.
// WARNING: IT WILL MAKE Node FAIL.
func NilRaftNodeRetrieval(id uint64) (api.Raft, error) {
	return api.Raft{}, errors.New("unimplemented")
}

// OnNewValue will be executed each time Raft node receives a new value.
// Optional.
type OnNewValue func(Item) error

// NilOnNewValue defines a null behaviour for OnNewVaue.
func NilOnNewValue(Item) error {
	return nil
}
