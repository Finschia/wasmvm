#!/bin/sh
set -e # Note we are not using bash here but the Alpine default shell

echo "Starting aarch64-unknown-linux-musl build"
export CC=/opt/aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc
cargo build --release --target aarch64-unknown-linux-musl --example staticlib
unset CC

echo "Starting x86_64-unknown-linux-musl build"
cargo build --release --target x86_64-unknown-linux-musl --example staticlib

cp target/aarch64-unknown-linux-musl/release/examples/libstaticlib.a artifacts/libwasmvm_static.aarch64.a
cp target/x86_64-unknown-linux-musl/release/examples/libstaticlib.a artifacts/libwasmvm_static.a
