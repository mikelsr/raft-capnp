using Go = import "/go.capnp";

@0x8dcfa60b52844164;

$Go.package("api");
$Go.import("github.com/mikelsr/raft-capnp/proto/api");

# TODO replace Raft with Node	s, err := f.Future.Struct()
interface Raft {
    join    @0 (node :Raft) -> (nodes :List(Raft), error :Text);
    leave   @1 (node :Raft) -> (error :Text);
    send    @2 (msg  :Data) -> (error :Text);
    put     @3 (item :Item) -> (error :Text);
    list    @4 ()           -> (objects :List(Item));
    members @5 ()           -> (members :List(Raft));
    id      @6 ()           -> (id :UInt64);
}

struct Item {
    key     @0 :Data;
    value   @1 :Data;
}

