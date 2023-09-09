package raft

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/mikelsr/raft-capnp/proto/api"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

// Node implements api.Raft_Server.
type Node struct {
	ID uint64
	*Cluster
	items  ItemMap
	queue  MessageQueue
	logger raft.Logger

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
}

func New() *Node {
	return &Node{
		ID:      DefaultID(),
		Cluster: NewCluster(),
		items:   ItemMap{},
		queue:   make(MessageQueue),
		logger:  DefaultLogger,

		pauseChan: make(chan bool),
		ticker:    *time.NewTicker(time.Second),
		stopChan:  make(chan error),
	}
}

// Cap instantiates a new capability from n.
func (n *Node) Cap() api.Raft {
	return api.Raft_ServerToClient(n)
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

	n.init()
	n.logger.Info("init done")

	var err error
	for {
		select {
		case <-n.ticker.C:
			n.logger.Info("tick")
			n.Raft.Tick()

		case ready := <-n.Raft.Ready():
			n.logger.Info("ready")
			err = n.doReady(ctx, ready)

		case pause := <-n.pauseChan:
			n.logger.Info("pause")
			err = n.doPause(ctx, pause)

		case <-ctx.Done():
			n.logger.Info("ctx done")
			err = ctx.Err()

		case err := <-n.stopChan:
			n.logger.Infof("stop with error: %s", err.Error())
			defer close(n.stopChan)
			defer n.doStop(ctx, err)
			return
		}

		if err != nil {
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
	n.Config.Logger = n.logger
}

// init the underlying Raft node and register self in cluster.
func (n *Node) init() {
	n.lateConfig()
	peers := []raft.Peer{{ID: n.ID}}
	n.Cluster.addPeer(n.ID, api.Raft_ServerToClient(n))
	for k := range n.Cluster.Peers() {
		if k == n.ID {
			continue
		}
		peers = append(peers, raft.Peer{ID: k})
	}
	n.Raft = raft.StartNode(n.Config, peers)
}

func (n *Node) doReady(ctx context.Context, ready raft.Ready) error {
	err := n.RaftStore(n.Storage, ready.HardState, ready.Entries, ready.Snapshot)
	if err != nil {
		return err
	}

	n.logger.Info("send messages")
	n.sendMessages(ctx, ready.Messages)

	if !raft.IsEmptySnap(ready.Snapshot) {
		return errors.New("snapshotting is not yet implemented")
	}

	n.logger.Info("process entries")
	for _, entry := range ready.CommittedEntries {
		switch entry.Type {
		case raftpb.EntryNormal:
			n.logger.Info("normal entry")
			err = n.addEntry(entry)
		case raftpb.EntryConfChange:
			n.logger.Info("conf change")
			err = n.addConfChange(ctx, entry)
		default:
			err = fmt.Errorf(
				"unrecognized entry type: %s", raftpb.EntryType_name[int32(entry.Type)])
		}
		if err != nil {
			return err
		}
		n.Raft.Advance()
	}
	return err
}

func (n *Node) doStop(ctx context.Context, err error) {
	if err != nil {
		log.Fatalf("Stop server with error `%s`.\n", err.Error())
	} else {
		log.Println("Stop server with no errors.")
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
		n.OnNewValue(*item)
	}

	n.items.Put(*item)

	return nil
}

func (n *Node) addConfChange(ctx context.Context, entry raftpb.Entry) error {
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
		n.logger.Info("add node")
		err = n.addNode(ctx, cc)
	case raftpb.ConfChangeRemoveNode:
		n.logger.Info("remove node")
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
	return n.Register(ctx, cc.NodeID)
}

func (n *Node) removeNode(ctx context.Context, cc raftpb.ConfChange) error {
	// Unregister the node.
	defer n.Unregister(ctx, cc.NodeID)

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
	peers := n.Cluster.Peers()

	for _, msg := range messages {
		// Recipient is send.
		if msg.To == n.ID {
			n.Raft.Step(ctx, msg)
			continue
		}

		// Recipient is potentially a peer.
		if peer, found := peers[msg.To]; found {
			if err := rpcSend(ctx, peer, msg); err != nil {
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
		nodeC <- node
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

	return node, err
}

// Register a new node in the cluster.
func (n *Node) Register(ctx context.Context, id uint64) error {
	var (
		err  error
		node api.Raft
	)

	n.logger.Info("retrieve with timeout")
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
	n.logger.Info("add peer")
	n.Cluster.AddPeer(ctx, node)
	return nil
}

// Unregister a node.
func (n *Node) Unregister(ctx context.Context, id uint64) {
	if n.ID == id {
		return
	}

	peer := n.Cluster.PopPeer(id)
	peer.Release()
}
