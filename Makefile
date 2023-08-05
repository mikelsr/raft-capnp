# Set a sensible default for the $GOPATH in case it's not exported.
# If you're seeing path errors, try exporting your GOPATH.
ifeq ($(origin GOPATH), undefined)
    GOPATH := $(HOME)/go
endif

all: capnp test


capnp: capnp-raft
# N.B.:  compiling capnp schemas requires having capnproto.org/go/capnp/v3 installed
#        on the GOPATH.

capnp-raft:
	@mkdir -p proto/api
	@capnp compile -I$(GOPATH)/src/capnproto.org/go/capnp/v3/std -ogo:proto/api --src-prefix=proto proto/raft.capnp

test: test-wasm

# Test that everything can be compiled to wasm
test-wasm:
	@env GOOS=wasip1 GOARCH=wasm gotip build -o ./test/wasm/test.wasm ./test/wasm
	@rm ./test/wasm/test.wasm