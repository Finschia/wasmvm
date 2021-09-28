//go:build linux && muslc && !static
// +build linux,muslc,!static

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm_muslc
import "C"
