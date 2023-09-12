# raft-capnp

Use Cap’n Proto as a transport for etcd's raft implementation.
Based on [abronan/proto](https://github.com/abronan/proton).

---

The Cap'n Proto definition provides methods to:

```python
interface Raft {
    add     @0 (node :Raft) -> (nodes :List(Raft), error :Text);
    # Propose adding $node to the Raft.
    remove  @1 (node :Raft) -> (error :Text);
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
```

---

Using the library:

```go
import (
    // for RPC calls
    "github.com/mikelsr/raft-capnp/proto/api"
    // for building and running nodes
    "github.com/mikelsr/raft-capnp/raft"
)
```

Users must define a function to retrieve a Raft node given an ID and configure
nodes to use it:

```go
type RaftNodeRetrieval func(context.Context, uint64) (api.Raft, error)
...

func retrieve(context.Context, uint64) (api.Raft, error) {
    ...
}
...
    node := raft.New().WithRaftNodeRetrieval(retrieve)
...
```

Check out the example at
[example/main.go](https://github.com/mikelsr/raft-capnp/blob/main/example/main.go).

