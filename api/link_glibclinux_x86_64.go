//go:build linux && !muslc && amd64
// +build linux,!muslc,amd64

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm.x86_64
import "C"
