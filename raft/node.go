package raft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mikelsr/raft-capnp/proto/api"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

// Node implements api.Raft_Server.
type Node struct {
	ID uint64
	Cluster
	items ItemMap

	// raft specifics
	raft raft.Node
	*raft.Config
	raft.Storage
	queue MessageQueue

	// status
	pause     bool // can it be done with atomic.Bool?
	pauseChan chan bool
	pauseLock sync.Mutex
	ticker    time.Ticker
	stopChan  chan error

	// externally defined functions
	OnNewValue
	RaftNodeRetrieval
	RaftStore
}

// CAPNP START

// Join a Raft cluster.
func (n *Node) Join(ctx context.Context, call api.Raft_join) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	node := call.Args().Node()
	nodes, err := n.join(ctx, node)
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
		caps.Set(i, node)
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
	var (
		msg *raftpb.Message
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

// CAPNP END

func (n *Node) Start(ctx context.Context) {
	var err error
	for {
		select {
		case <-n.ticker.C:
			n.raft.Tick()

		case ready := <-n.raft.Ready():
			err = n.doReady(ctx, ready)

		case pause := <-n.pauseChan:
			err = n.doPause(ctx, pause)

		case <-ctx.Done():
			err = ctx.Err()

		case err := <-n.stopChan:
			defer close(n.stopChan)
			defer n.doStop(ctx, err)
			return
		}

		if err != nil {
			n.stopChan <- err
		}
	}
}

func (n *Node) doReady(ctx context.Context, ready raft.Ready) error {
	err := n.RaftStore(n.Storage, ready.HardState, ready.Entries, ready.Snapshot)
	if err != nil {
		return err
	}

	n.broadcast(ctx, ready.Messages)

	if !raft.IsEmptySnap(ready.Snapshot) {
		return errors.New("snapshotting is not yet implemented")
	}

	for _, entry := range ready.CommittedEntries {
		switch entry.Type {
		case raftpb.EntryNormal:
			err = n.addEntry(entry)
		case raftpb.EntryConfChange:
			err = n.addConfChange(entry)
		default:
			err = fmt.Errorf(
				"unrecognized entry type: %s", raftpb.EntryType_name[int32(entry.Type)])
		}
		if err != nil {
			return err
		}
	}
	return err
}

func (n *Node) doStop(ctx context.Context, err error) {
	if err != nil {
		log.Fatalf("Stop server with error `%s`.\n", err.Error())
	} else {
		log.Println("Stop server with no errors.")
	}
	n.raft.Stop()
}

func (n *Node) doPause(ctx context.Context, pause bool) error {
	// wait until unpause
	n.setPaused(pause)
	err := n.waitPause(ctx)
	if err != nil {
		return err
	}
	// process pending messages
	return n.churnQueue(ctx)
}

// wait until pause is set to true.
func (n *Node) waitPause(ctx context.Context) error {
	for n.pause {
		select {
		case pause := <-n.pauseChan:
			n.setPaused(pause)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// churnQueue process all messages in the queue until there are none left.
func (n *Node) churnQueue(ctx context.Context) error {
	n.pauseLock.Lock()
	defer n.pauseLock.Unlock()

	for len(n.queue) > 0 {
		msg := <-n.queue
		err := n.raft.Step(ctx, msg)
		if err != nil {
			return err
		}
	}

	return nil
}

// Pause the raft node.
func (n *Node) Pause() {
	n.pauseChan <- true
}

// Resume the raft node.
func (n *Node) Resume() {
	n.pauseChan <- false
}

func (n *Node) IsPaused() bool {
	n.pauseLock.Lock()
	defer n.pauseLock.Unlock()
	return n.pause
}

func (n *Node) setPaused(pause bool) {
	n.pauseLock.Lock()
	defer n.pauseLock.Unlock()
	n.pause = pause
}

func (n *Node) addEntry(entry raftpb.Entry) error {
	if entry.Type != raftpb.EntryNormal || entry.Data == nil {
		return nil
	}

	item := &Item{}
	if err := item.Unmarshal(entry.Data); err != nil {
		return err
	}

	if n.OnNewValue != nil {
		n.OnNewValue(*item)
	}

	n.items.Put(*item)

	return nil
}

func (n *Node) addConfChange(entry raftpb.Entry) error {
	if entry.Type != raftpb.EntryConfChange || entry.Data == nil {
		return nil
	}

	var (
		err error
		cc  raftpb.ConfChange
	)

	if err = cc.Unmarshal(entry.Data); err != nil {
		return err
	}

	switch cc.Type {
	case raftpb.ConfChangeAddNode:

	case raftpb.ConfChangeRemoveNode:

	default:
		err = fmt.Errorf(
			"unrecognized conf change type: %s",
			raftpb.ConfChangeType_name[int32(cc.Type)])
	}
	return err
}

// TODO find a more appropiate name
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

func (n *Node) retrieve(id uint64, nodeC chan api.Raft, errC chan error) {
	node, err := n.RaftNodeRetrieval(id)
	if err != nil {
		errC <- err
	} else {
		nodeC <- node
	}
}

func (n *Node) retrieveWithTimeout(id uint64, timeout time.Duration) (api.Raft, error) {
	var (
		err  error
		node api.Raft
	)
	errC := make(chan error, 1)
	nodeC := make(chan api.Raft, 1)

	select {
	case node = <-nodeC:
		break
	case err = <-errC:
		break
	case <-time.After(timeout):
		err = fmt.Errorf("timed out after %s", timeout.String())
	}
	return node, err
}

// Register a new Node in the cluster.
func (n *Node) Register(ctx context.Context, id uint64) error {
	var (
		err  error
		node api.Raft
	)

	for i := 0; i < RetrievalRetries; i++ {
		node, err = n.retrieveWithTimeout(id, RetrievalTimeout)
		if err == nil {
			break
		}
	}

	if err != nil {
		return err
	}

	n.Cluster.AddPeer(ctx, node)
	return nil
}
