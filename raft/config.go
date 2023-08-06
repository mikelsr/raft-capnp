package raft

import (
	"log"
	"os"

	"go.etcd.io/raft/v3"
)

var defaultLogger = &raft.DefaultLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)}

func DefaultConfig() *raft.Config {
	return &raft.Config{
		HeartbeatTick:   HeartbeatTick,
		ElectionTick:    ElectionTick,
		MaxSizePerMsg:   MaxSizePerMsg,
		MaxInflightMsgs: MaxInflightMsgs,
		Logger:          defaultLogger,
	}
}
