package raft

import (
	"math"
	"time"
)

const (
	// Raft configuration
	HeartbeatTick   = 1
	ElectionTick    = 3
	MaxSizePerMsg   = math.MaxInt16
	MaxInflightMsgs = 256

	// Timeouts
	RetrievalTimeout = 10 * time.Second
	RetrievalRetries = 3
)
