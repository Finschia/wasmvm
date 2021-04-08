#!/bin/sh

git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
cargo build --release --example muslc
cp /code/target/release/examples/libmuslc.a /code/api/libwasmvm_muslc.a
