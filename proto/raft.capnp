using Go = import "/go.capnp";

@0x8dcfa60b52844164;

$Go.package("api");
$Go.import("github.com/mikelsr/raft-capnp/proto/api");

# TODO replace Raft with Node
interface Raft {
    add    @0 (node :Raft) -> (nodes :List(Raft), error :Text);
    # Propose adding $node to the Raft.
    remove   @1 (node :Raft) -> (error :Text);
    # Propose removing $node from the Raft.
    send    @2 (msg  :Data) -> (error :Text);
    # Send a message to the Raft node.
    put     @3 (item :Item) -> (error :Text);
    # Put an k/v pair on the cluster.
    items   @4 ()           -> (objects :List(Item));
    # List every k/v pair in the cluster.
    members @5 ()           -> (members :List(Raft));
    # List every memeber of the cluster.
    id      @6 ()           -> (id :UInt64);
    # Get the Raft ID of the node.
}

struct Item {
    key     @0 :Data;
    value   @1 :Data;
}

