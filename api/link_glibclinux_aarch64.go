//go:build linux && !static && arm64
// +build linux,!static,arm64

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm.aarch64
import "C"
