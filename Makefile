# Set a sensible default for the $GOPATH in case it's not exported.
# If you're seeing path errors, try exporting your GOPATH.
ifeq ($(origin GOPATH), undefined)
    GOPATH := $(HOME)/go
endif

all: capnp


capnp: capnp-raft
# N.B.:  compiling capnp schemas requires having capnproto.org/go/capnp/v3 installed
#        on the GOPATH.

capnp-raft:
	@mkdir -p proto/api
	@capnp compile -I$(GOPATH)/src/capnproto.org/go/capnp/v3/std -ogo:proto/api --src-prefix=proto proto/raft.capnp
