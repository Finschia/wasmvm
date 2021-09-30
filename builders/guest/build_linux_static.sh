#!/bin/sh

TARGET_NAME="${1:-static}"

cargo build --release --example staticlib
cp /code/target/release/examples/libstaticlib.a "/code/api/libwasmvm_$TARGET_NAME.a"
