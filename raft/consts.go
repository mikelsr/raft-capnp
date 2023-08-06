package raft

import "math"

const (
	HeartbeatTick   = 1
	ElectionTick    = 3
	MaxSizePerMsg   = math.MaxInt16
	MaxInflightMsgs = 256
)
