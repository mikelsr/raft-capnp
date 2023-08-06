package raft

import (
	"context"
	"encoding/json"

	"github.com/mikelsr/raft-capnp/proto/api"
	"go.etcd.io/raft/v3/raftpb"
)

/*
	Everything in this file should be ported to a proper library to abstract from
	capnp usage.
*/

func rpcGetId(ctx context.Context, n api.Raft) (uint64, error) {
	f, release := n.Id(ctx, nil)
	defer release()
	<-f.Done()
	results, err := f.Struct()
	if err != nil {
		return 0, err
	}
	return results.Id(), nil
}

func rpcSend(ctx context.Context, n api.Raft, msg raftpb.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	f, release := n.Send(ctx, func(r api.Raft_send_Params) error {
		return r.SetMsg(data)
	})
	defer release()
	<-f.Done()
	_, err = f.Struct()
	return err
}
