package raft

import (
	"context"
	"io"

	"github.com/mikelsr/raft-capnp/proto/api"
)

type Info struct {
	ID   uint64
	Chan io.ReadWriteCloser
}

func (i Info) Id(ctx context.Context, call api.NodeInfo_id) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	res.SetId(i.ID)
	return nil
}

// TODO mikel
func (i Info) Channel(ctx context.Context, call api.NodeInfo_channel) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	return res.SetChan("chan")
}

func (i Info) Cap() api.NodeInfo {
	return api.NodeInfo_ServerToClient(i)
}
