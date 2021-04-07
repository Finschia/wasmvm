#!/bin/bash

git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
cargo build --release
cp target/release/deps/libwasmvm.so api
# FIXME: re-enable stripped so when we approach a production release, symbols are nice for debugging
# strip api/libwasmvm.so
