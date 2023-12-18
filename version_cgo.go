//go:build cgo

package cosmwasm

import (
	"github.com/Finschia/wasmvm/internal/api"
)

func libwasmvmVersionImpl() (string, error) {
	return api.LibwasmvmVersion()
}
