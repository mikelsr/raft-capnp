using Go = import "/go.capnp";

@0x8dcfa60b52844164;

$Go.package("api");
$Go.import("github.com/mikelsr/raft-capnp/proto/api");

interface Raft {
    join        @0 (nodeInfo :NodeInfo) -> (nodes :List(NodeInfo), error :Text);
    leave       @1 (nodeInfo :NodeInfo) -> (error :Text);
    send        @2 (msg :Data)          -> (error :Text);
    put         @3 (item :Item)         -> (error :Text);
    list        @4 ()                   -> (objects :List(Item));
    members     @5 ()                   -> (members :List(NodeInfo));
}

struct Item {
    key     @0 :Data;
    value   @1 :Data;
}

interface NodeInfo {
    id      @0 () -> (id :UInt64);
    channel @1 () -> (chan :Text);  # TODO replace with WW channel
}
