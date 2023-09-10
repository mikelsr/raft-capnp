package raft

import (
	"context"
	"encoding/json"

	"github.com/mikelsr/raft-capnp/proto/api"
	"go.etcd.io/raft/v3/raftpb"
)

// Add a node to the Raft cluster.
func (n *Node) Add(ctx context.Context, call api.Raft_add) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	node := call.Args().Node()
	nodes, err := n.add(ctx, node)
	if err != nil {
		res.SetError(err.Error())
		return err
	}
	caps, err := res.NewNodes(int32(len(nodes)))
	if err != nil {
		res.SetError(err.Error())
		return err
	}
	for i, node := range nodes {
		caps.Set(i, node.AddRef())
	}
	if err = res.SetNodes(caps); err != nil {
		res.SetError(err.Error())
		return err
	}
	return nil
}

// add is the capnp-free logic of Add.
func (n *Node) add(ctx context.Context, node api.Raft) ([]api.Raft, error) {
	node = node.AddRef()

	if err := node.Resolve(ctx); err != nil {
		return nil, err
	}

	id, err := rpcGetId(ctx, node.AddRef())
	if err != nil {
		return nil, err
	}

	cc := raftpb.ConfChange{
		ID:     id,
		Type:   raftpb.ConfChangeAddNode,
		NodeID: id,
	}

	n.Logger.Debugf("[%x] joined by peer %x\n", n.ID, id)

	if err := n.Raft.ProposeConfChange(ctx, cc); err != nil {
		return nil, err
	}

	// Return known peers
	pm := n.View.Peers()
	peers := make([]api.Raft, len(pm))
	i := 0
	for _, peer := range pm {
		peers[i] = peer.AddRef()
		i++
	}

	return peers, nil
}

// Remove a node from the cluster.
func (n *Node) Remove(ctx context.Context, call api.Raft_remove) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	node := call.Args().Node()
	if err = n.remove(ctx, node.AddRef()); err != nil {
		res.SetError(err.Error())
		return err
	}
	return nil
}

// remove is the capnp-free logic of Remove.
func (n *Node) remove(ctx context.Context, node api.Raft) error {
	id, err := rpcGetId(ctx, node.AddRef())
	if err != nil {
		return err
	}

	cc := raftpb.ConfChange{
		ID:     id,
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: id,
	}

	n.Logger.Debugf("[%x] left by peer %x\n", id)

	return n.Raft.ProposeConfChange(ctx, cc)
}

// Send advances the raft state machine with the received message.
func (n *Node) Send(ctx context.Context, call api.Raft_send) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	msgData, err := call.Args().Msg()
	if err != nil {
		res.SetError(err.Error())
		return err
	}
	if err = n.send(ctx, msgData); err != nil {
		res.SetError(err.Error())
		return err
	}
	return nil
}

// send is the capnp-free logic of Send.
func (n *Node) send(ctx context.Context, msgData []byte) error {
	var (
		msg = &raftpb.Message{}
		err error
	)

	if err = json.Unmarshal(msgData, msg); err != nil {
		return err
	}

	if n.IsPaused() {
		n.pauseLock.Lock()
		n.queue <- *msg
		n.pauseLock.Unlock()
	} else {
		err = n.Raft.Step(ctx, *msg)
	}
	return err
}

// Put proposes a new value.
func (n *Node) Put(ctx context.Context, call api.Raft_put) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}

	itemCap, err := call.Args().Item()
	if err != nil {
		res.SetError(err.Error())
		return err
	}

	item, err := ItemFromApi(itemCap)
	if err != nil {
		return err
	}

	if err = n.put(ctx, item); err != nil {
		res.SetError(err.Error())
		return err
	}

	return nil
}

// put is the capnp-free logic of Put.
func (n *Node) put(ctx context.Context, item Item) error {
	itemData, err := item.Marshal()
	if err != nil {
		return err
	}

	return n.Raft.Propose(ctx, itemData)
}

func (n *Node) Items(ctx context.Context, call api.Raft_items) error {
	return nil
}

func (n *Node) Members(ctx context.Context, call api.Raft_members) error {
	return nil
}

func (n *Node) Id(ctx context.Context, call api.Raft_id) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	res.SetId(n.ID)
	return nil
}
