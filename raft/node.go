package raft

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/mikelsr/raft-capnp/proto/api"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

// Node implements api.Raft_Server.
type Node struct {
	*View
	Map    ItemMap
	queue  MessageQueue
	Logger raft.Logger

	// Raft specifics
	Raft raft.Node
	raft.Storage
	*raft.Config

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

	init bool
}

// Node constructor. Call Node.WithX methods for configuration.
func New() *Node {
	return &Node{
		View:   NewView(),
		Map:    ItemMap{},
		queue:  make(MessageQueue),
		Logger: DefaultLogger(false),

		pauseChan: make(chan bool),
		ticker:    *time.NewTicker(time.Second),
		stopChan:  make(chan error),
	}
}

// Cap instantiates a new Raft capability from the node.
func (n *Node) Cap() api.Raft {
	return api.Raft_ServerToClient(n).AddRef()
}

// Stop a node in a non-forcing, non way (pending operations will complete).
// Non-blocking.
// To stop forcefully, cancel the context Node.Start() was called with.
func (n *Node) Stop(cause error) {
	if cause == nil {
		_, file, no, ok := runtime.Caller(1)
		if ok {
			cause = fmt.Errorf("manually stopped from %s#%d", file, no)
		} else {
			cause = errors.New("manually stopped")
		}
	}
	n.stopChan <- cause
}

// Start the raft node.
func (n *Node) Start(ctx context.Context) {

	n.Init()

	var err error
	for {
		select {
		case <-n.ticker.C:
			n.Raft.Tick()

		case ready := <-n.Raft.Ready():
			err = n.doReady(ctx, ready)

		case pause := <-n.pauseChan:
			err = n.doPause(ctx, pause)

		case <-ctx.Done():
			n.Logger.Debug("ctx done")
			err = ctx.Err()

		case err := <-n.stopChan:
			n.Logger.Debugf("stop with error: %s", err.Error())
			defer close(n.stopChan)
			defer n.doStop(ctx, err)
			return
		}

		if err != nil {
			n.Logger.Error(err)
			go func() {
				n.stopChan <- err
			}()
		}
	}
}

// lateConfig performs configuration steps that cannot be done
// at construction but need to be done before Start.
func (n *Node) lateConfig() {
	n.Config.ID = n.ID
	n.Config.Storage = n.Storage
	n.Config.Logger = n.Logger
}

// Init the underlying Raft node and register self in cluster.
func (n *Node) Init() {
	if !n.init {
		n.init = true
		n.lateConfig()
		peerList := []raft.Peer{{ID: n.ID}}
		n.Raft = raft.StartNode(n.Config, peerList)
		n.Logger.Debug("init done")
	}
}

func (n *Node) doReady(ctx context.Context, ready raft.Ready) error {
	err := n.RaftStore(n.Storage, ready.HardState, ready.Entries, ready.Snapshot)
	if err != nil {
		return err
	}

	n.sendMessages(ctx, ready.Messages)

	if !raft.IsEmptySnap(ready.Snapshot) {
		return errors.New("snapshotting is not yet implemented")
	}

	for _, entry := range ready.CommittedEntries {
		switch entry.Type {
		case raftpb.EntryNormal:
			n.Logger.Debug("process normal entry")
			err = n.addEntry(entry)
		case raftpb.EntryConfChange:
			n.Logger.Debug("process conf change")
			err = n.addConfChange(ctx, entry)
		default:
			err = fmt.Errorf(
				"unrecognized entry type: %s", raftpb.EntryType_name[int32(entry.Type)])
		}
		if err != nil {
			return err
		}
	}
	n.Raft.Advance()
	return err
}

func (n *Node) doStop(ctx context.Context, err error) {
	if err != nil {
		n.Logger.Fatalf("Stop server with error `%s`.\n", err.Error())
	} else {
		n.Logger.Debug("Stop server with no errors.")
	}
	n.Raft.Stop()
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
		err := n.Raft.Step(ctx, msg)
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
		if err := n.OnNewValue(*item); err != nil {
			return err
		}
	}

	n.Map.Put(*item)

	return nil
}

func (n *Node) addConfChange(ctx context.Context, entry raftpb.Entry) error {
	if entry.Data == nil {
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
		n.Logger.Debugf("[%x] add node %x\n", n.ID, cc.NodeID)
		err = n.addNode(ctx, cc)
	case raftpb.ConfChangeRemoveNode:
		n.Logger.Debug("remove node")
		err = n.removeNode(ctx, cc)
	default:
		err = fmt.Errorf(
			"unrecognized conf change type: %s",
			raftpb.ConfChangeType_name[int32(cc.Type)])
	}
	n.Raft.ApplyConfChange(cc)
	return err
}

func (n *Node) addNode(ctx context.Context, cc raftpb.ConfChange) error {
	return n.register(ctx, cc.NodeID)
}

func (n *Node) removeNode(ctx context.Context, cc raftpb.ConfChange) error {
	// Unregister the node.
	defer n.unregister(ctx, cc.NodeID)

	// Leader, self, steps down.
	if n.ID == cc.NodeID && n.ID == n.Raft.Status().Lead {
		n.Raft.Stop()
		return nil
	}
	// Other leader steps down.
	if cc.NodeID == n.Raft.Status().Lead {
		return n.Raft.Campaign(ctx)
	}
	return nil
}

// TODO find a more appropiate name
func (n *Node) sendMessages(ctx context.Context, messages []raftpb.Message) {
	peers := n.View.Peers()

	for _, msg := range messages {

		// Recipient is sender.
		if msg.To == n.ID {
			n.Raft.Step(ctx, msg)
			continue
		}

		// Recipient is potentially a peer.
		if peer, found := peers[msg.To]; found {
			n.Logger.Debugf("[%x] send message %v to peer %x\n", n.ID, msg, msg.To)
			if err := rpcSend(ctx, peer.AddRef(), msg); err != nil {
				n.Logger.Error(err)
				n.Raft.ReportUnreachable(msg.To)
			}
		}
	}
}

func (n *Node) retrieve(ctx context.Context, id uint64, nodeC chan api.Raft, errC chan error) {
	node, err := n.RaftNodeRetrieval(ctx, id)
	if err != nil {
		errC <- err
	} else {
		nodeC <- node.AddRef()
	}
}

func (n *Node) retrieveWithTimeout(ctx context.Context, id uint64, timeout time.Duration) (api.Raft, error) {
	var (
		err  error
		node api.Raft
	)
	errC := make(chan error, 1)
	nodeC := make(chan api.Raft, 1)

	// Add a cancel function for timeout/context cases.
	rCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go n.retrieve(rCtx, id, nodeC, errC)

	select {
	case node = <-nodeC:
		break
	case err = <-errC:
		break
	case <-time.After(timeout):
		err = fmt.Errorf("timed out after %s", timeout.String())
	case <-ctx.Done():
		err = ctx.Err()
	}

	return node.AddRef(), err
}

// register a new node in the cluster.
func (n *Node) register(ctx context.Context, id uint64) error {
	var (
		err  error
		node api.Raft
	)

	for i := 0; i < RetrievalRetries; i++ {
		node, err = n.retrieveWithTimeout(ctx, id, RetrievalTimeout)
		if err == nil {
			break
		}
		if err != nil && err == ctx.Err() {
			break
		}
	}

	if err != nil {
		return err
	}
	n.Logger.Debugf("[%x] add peer %x\n", n.ID, id)
	n.View.AddPeer(ctx, node.AddRef())
	return nil
}

// unregister a node.
func (n *Node) unregister(ctx context.Context, id uint64) {
	n.Logger.Debugf("[%x] unregister node %x\n", n.ID, id)
	if n.ID == id {
		return
	}

	peer := n.View.PopPeer(id)
	peer.Release()
}
