# Cross Compilation Scripts

As this library is targetting go developers, we cannot assume a properly set up
rust environment on their system. Further, when importing this library, there is
no clean way to add a `libwasmvm.{so,dll,dylib}`. It needs to be committed with
the tagged (go) release in order to be easily usable.

The solution is to precompile the rust code into libraries for the major
platforms (Linux, Windows, MacOS) and commit them to the repository at each
tagged release. This should be doable from one host machine, but is a bit
tricky. This folder contains build scripts and a Docker image to create all
dynamic libraries from one host. In general this is set up for a Linux host, but
any machine that can run Docker can do the cross-compilation.

For more information visit [here](https://github.com/CosmWasm/wasmvm/tree/main/builders#readme).
