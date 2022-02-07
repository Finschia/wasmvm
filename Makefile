.PHONY: all build build-rust build-go test

BUILDERS_PREFIX := line/wasmvm-builder
USER_ID := $(shell id -u)
USER_GROUP = $(shell id -g)

SHARED_LIB_EXT = "" # File extension of the shared library
ifeq ($(OS),Windows_NT)
	SHARED_LIB_EXT = dll
else
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Linux)
		SHARED_LIB_EXT = so
	endif
	ifeq ($(UNAME_S),Darwin)
		SHARED_LIB_EXT = dylib
	endif
endif

all: build test

build: build-rust build-go

build-rust: build-rust-release

# Use debug build for quick testing.
# In order to use "--features backtraces" here we need a Rust nightly toolchain, which we don't have by default
build-rust-debug:
	(cd libwasmvm && cargo build)
	cp libwasmvm/target/debug/libwasmvm.$(SHARED_LIB_EXT) api
	make update-bindings

# use release build to actually ship - smaller and much faster
#
# See https://github.com/CosmWasm/wasmvm/issues/222#issuecomment-880616953 for two approaches to
# enable stripping through cargo (if that is desired).
build-rust-release:
	(cd libwasmvm && cargo build --release)
	cp libwasmvm/target/release/libwasmvm.$(SHARED_LIB_EXT) api
	make update-bindings
	@ #this pulls out ELF symbols, 80% size reduction!

build-go:
	go build ./...

test:
	RUST_BACKTRACE=1 go test -v ./api ./types . -tags mocks

test-safety:
	GODEBUG=cgocheck=2 go test -race -v -count 1 ./api -tags mocks

# Creates a release build in a containerized build environment of the static library for Alpine Linux (.a)
release-build-alpine:
	rm -rf libwasmvm/target/release
	# build the muslc *.a file
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd)/libwasmvm:/code $(BUILDERS_PREFIX):alpine
	cp libwasmvm/target/release/examples/libstaticlib.a api/libwasmvm_static.a
	make update-bindings
	# try running go tests using this lib with muslc
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/testing -w /testing $(BUILDERS_PREFIX):alpine go build -tags static .
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/testing -w /testing $(BUILDERS_PREFIX):alpine go test -tags 'static mocks' ./api ./types

# Creates a release build in a containerized build environment of the static library for glibc Linux (.a)
release-build-linux-static:
	rm -rf libwasmvm/target/release
	# build the glibc *.a file
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd)/libwasmvm:/code $(BUILDERS_PREFIX):static
	cp libwasmvm/target/release/examples/libstaticlib.a api/libwasmvm_static.a
	make update-bindings
	# try running go tests using this lib with glibc
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/testing -w /testing $(BUILDERS_PREFIX):static go build -tags static .
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/testing -w /testing $(BUILDERS_PREFIX):static go test -tags='static mocks' ./api ./types

# Creates a release build in a containerized build environment of the shared library for glibc Linux (.so)
release-build-linux:
	rm -rf libwasmvm/target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd)/libwasmvm:/code $(BUILDERS_PREFIX):centos7
	cp libwasmvm/target/release/deps/libwasmvm.so api
	make update-bindings

# Creates a release build in a containerized build environment of the shared library for macOS (.dylib)
release-build-macos:
	rm -rf libwasmvm/target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd)/libwasmvm:/code $(BUILDERS_PREFIX):cross
	cp libwasmvm/target/x86_64-apple-darwin/release/deps/libwasmvm.dylib api
	cp libwasmvm/bindings.h api
	make update-bindings

update-bindings:
	# After we build libwasmvm, we have to copy the generated bindings for Go code to use.
	# We cannot use symlinks as those are not reliably resolved by `go get` (https://github.com/CosmWasm/wasmvm/pull/235).
	cp libwasmvm/bindings.h api

release-build:
	# Write like this because those must not run in parallel
	make release-build-alpine
	make release-build-linux
	make release-build-linux-static
	make release-build-macos

test-alpine: release-build-alpine
	# build a go binary
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/testing -w /testing $(BUILDERS_PREFIX):alpine go build -tags 'static mocks' -o demo ./cmd
	# run static binary in an alpine machines (not dlls)
	docker run --rm --read-only -v $(shell pwd):/testing -w /testing alpine:3.14 ./demo ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/testing -w /testing alpine:3.13 ./demo ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/testing -w /testing alpine:3.12 ./demo ./api/testdata/hackatom.wasm
	docker run --rm --read-only -v $(shell pwd):/testing -w /testing alpine:3.11 ./demo ./api/testdata/hackatom.wasm
	# run static binary locally if you are on Linux
	# ./muslc.exe ./api/testdata/hackatom.wasm

test-static: release-build-linux-static
	# build a go binary
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code $(BUILDERS_PREFIX):static go build -tags='static mocks' -o static.exe ./cmd
	# run static binary in an alpine machines (not dlls)
	docker run --rm --read-only -v $(shell pwd):/code -w /code centos ./static.exe ./api/testdata/hackatom.wasm
	# run static binary locally if you are on Linux
	# ./static.exe ./api/testdata/hackatom.wasm
