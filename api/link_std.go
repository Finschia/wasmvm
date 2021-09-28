//go:build (linux && !muslc && !static) || darwin
// +build linux,!muslc,!static darwin

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm
import "C"
