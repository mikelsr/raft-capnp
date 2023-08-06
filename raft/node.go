package raft

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/mikelsr/raft-capnp/proto/api"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

// Node implements api.Raft_Server.
type Node struct {
	ID uint64
	Cluster

	// raft specifics
	raft    raft.Node
	storage Storage
	queue   MessageQueue

	// status
	pause     bool // can it be done with atomic.Bool?
	pauseChan chan bool
	pauseLock sync.Mutex
	ticker    time.Ticker
	stopChan  chan error

	hook OnNewValue
}

// Join a Raft cluster.
func (n *Node) Join(ctx context.Context, call api.Raft_join) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	nodes, err := n.join(ctx, n.Info())
	if err != nil {
		res.SetError(err.Error())
		return err
	}
	caps, err := res.NewNodes(int32(len(nodes)))
	if err != nil {
		res.SetError(err.Error())
		return err
	}
	for i, n := range nodes {
		caps.Set(i, n.Cap())
	}
	if err = res.SetNodes(caps); err != nil {
		res.SetError(err.Error())
		return err
	}
	return nil
}

// join is the capnp-free logic of Join.
func (n *Node) join(ctx context.Context, node api.Raft) ([]api.Raft, error) {

	id, err := rpcGetId(ctx, node)
	if err != nil {
		return nil, err
	}

	cc := raftpb.ConfChange{
		ID:     id,
		Type:   raftpb.ConfChangeAddNode,
		NodeID: id,
		// TODO con can nodes be propagated?
		// Context: marshaledCap,
	}

	if err := n.raft.ProposeConfChange(ctx, cc); err != nil {
		return nil, err
	}

	// TODO is the map necessary?
	peers := n.Cluster.Peers()
	nodes := make([]api.Raft, len(peers))
	i := 0
	for _, node := range peers {
		nodes[i] = node
		i++
	}

	return nodes, nil
}

// Leave a Raft cluster.
func (n *Node) Leave(ctx context.Context, call api.Raft_leave) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	node := call.Args().Node()
	if err = n.leave(ctx, node); err != nil {
		res.SetError(err.Error())
		return err
	}
	return nil
}

// leave is the capnp-free logic of Leave.
func (n *Node) leave(ctx context.Context, node api.Raft) error {
	id, err := rpcGetId(ctx, node)
	if err != nil {
		return err
	}

	cc := raftpb.ConfChange{
		ID:     id,
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: id,
	}
	return n.raft.ProposeConfChange(ctx, cc)
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
	var msg *raftpb.Message
	var err error

	if err = json.Unmarshal(msgData, msg); err != nil {
		return err
	}

	if n.isPaused() {
		n.pauseLock.Lock()
		n.queue = append(n.queue, *msg)
		n.pauseLock.Unlock()
	} else {
		err = n.raft.Step(ctx, *msg)
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

	return n.raft.Propose(ctx, itemData)
}

func (n *Node) List(ctx context.Context, call api.Raft_list) error {
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

// TODO: send in the original repo
func broadcast() {}

func (n *Node) setPaused(pause bool) {
	n.pauseLock.Lock()
	defer n.pauseLock.Unlock()
	n.pause = pause
	if n.queue == nil {
		n.queue = make([]raftpb.Message, 0)
	}
}

func (n *Node) isPaused() bool {
	n.pauseLock.Lock()
	defer n.pauseLock.Unlock()
	return n.pause
}

func (n *Node) setPaused(pause bool) {
	n.pauseLock.Lock()
	defer n.pauseLock.Unlock()
	n.pause = pause
}

// TODO: send in the original repo
func (n *Node) broadcast(ctx context.Context, messages []raftpb.Message) {
	peers := n.Cluster.Peers()

	for _, msg := range messages {
		// Recipient is send.
		if msg.To == n.ID {
			n.raft.Step(ctx, msg)
			continue
		}

		// Recipient is potentially a peer.
		if peer, found := peers[msg.To]; found {
			if err := rpcSend(ctx, peer, msg); err != nil {
				n.raft.ReportUnreachable(msg.To)
			}
		}
	}
}

// store a log entry.
func (n *Node) store(hardState raftpb.HardState, entries []raftpb.Entry, snapshot raftpb.Snapshot) {
	n.storage.Append(entries)

	if !raft.IsEmptyHardState(hardState) {
		n.storage.SetHardState(hardState)
	}

	if !raft.IsEmptySnap(snapshot) {
		n.storage.ApplySnapshot(snapshot)
	}
}
