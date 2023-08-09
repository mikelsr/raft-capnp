# Set a sensible default for the $GOPATH in case it's not exported.
# If you're seeing path errors, try exporting your GOPATH.
ifeq ($(origin GOPATH), undefined)
    GOPATH := $(HOME)/go
endif

# Use gotip if available
ifeq (, $(shell which gotip))
	GO := go
else
	GO := gotip
endif

api_dir := ./proto/api
example_file := ./example/example
test_dir := ./test/
wasm_dir := ${test_dir}/wasm
wasm_file := ${wasm_dir}/test.wasm

.PHONY: all test clean example

all: capnp example test

clean: capnp-clean example-clean test-clean


capnp: capnp-raft
# N.B.:  compiling capnp schemas requires having capnproto.org/go/capnp/v3 installed
#        on the GOPATH.

capnp-raft:
	@mkdir -p ${api_dir}
	@capnp compile -I$(GOPATH)/src/capnproto.org/go/capnp/v3/std -ogo:${api_dir} --src-prefix=proto proto/raft.capnp

capnp-clean:
	@rm -rf ${api_dir}

example:
	@${GO} build -o ${example_file} ./example

example-clean:
	@rm -rf ${example_file}

test: test-wasm example example-clean

# Test that everything can be compiled to wasm
test-wasm:
	@mkdir -p ${wasm_dir}
	@env GOOS=wasip1 GOARCH=wasm ${GO} build -o ${wasm_file} ./example
	@rm -rf ${wasm_dir}

test-clean:
	@rm -rf ${wasm_dir}
