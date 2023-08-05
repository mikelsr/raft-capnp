package raft

import (
	"errors"

	"capnproto.org/go/capnp/v3"
	"github.com/mikelsr/raft-capnp/proto/api"
)

// Cap is a Capnp capability.
type Cap interface {
	api.Raft | api.NodeInfo
}

// PointerList creates a list of pointers.
func PointerList[C Cap, CS any](caps []CS, serverToClient func(CS) C) (capnp.PointerList, error) {
	_, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return capnp.PointerList{}, err
	}
	l, err := capnp.NewPointerList(seg, int32(len(caps)))
	if err != nil {
		return capnp.PointerList{}, err
	}

	for i, cap := range caps {
		client := capnp.Client(serverToClient(cap))
		if !client.IsValid() {
			return capnp.PointerList{}, errors.New("invalid capability when converting to list")
		}
		_, iSeg, err := capnp.NewMessage(capnp.SingleSegment(nil))
		if err != nil {
			return capnp.PointerList{}, err
		}
		l.Set(i, client.EncodeAsPtr(iSeg))
	}
	return l, nil
}
