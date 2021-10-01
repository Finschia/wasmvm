//go:build linux && static
// +build linux,static

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm_static -lm -ldl
import "C"
