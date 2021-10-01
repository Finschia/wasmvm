#!/bin/sh

cargo build --release --example staticlib
cp /code/target/release/examples/libstaticlib.a "/code/api/libwasmvm_static.a"
