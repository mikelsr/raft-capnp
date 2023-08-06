package raft

import "go.etcd.io/raft/v3/raftpb"

type MessageQueue chan raftpb.Message
