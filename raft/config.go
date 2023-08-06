package raft

import "go.etcd.io/raft/v3"

func (n *Node) WithID(id uint64) *Node {
	n.ID = id
	return n
}

func (n *Node) WithOnNewValue(f OnNewValue) *Node {
	n.OnNewValue = f
	return n
}

func (n *Node) WithRaftNodeRetrieval(f RaftNodeRetrieval) *Node {
	n.RaftNodeRetrieval = f
	return n
}

func (n *Node) WithRaftStore(f RaftStore) *Node {
	n.RaftStore = f
	return n
}

func (n *Node) WithStorage(storage raft.Storage) *Node {
	n.Storage = storage
	return n
}

func (n *Node) WithRaftConfig(config *raft.Config) *Node {
	n.Config = config
	return n
}
