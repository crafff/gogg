package cache

import "encoding/json"

// Thin wrappers over encoding/json so the tests' memCache can share
// the exact same serialisation path as Redis. If we ever switch to
// a faster codec (sonic, ffjson) it's a one-file diff.

func jsonMarshal(v any) ([]byte, error)   { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
