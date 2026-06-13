package riotapi

import (
	"io"

	"github.com/bytedance/sonic"
)

func decodeJSON(r io.Reader, v any) error {
	return sonic.ConfigDefault.NewDecoder(r).Decode(v)
}
