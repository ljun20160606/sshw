package multiplex

import (
	"encoding/json"
)

type Request struct {
	Path string
	Body json.RawMessage

	// reader
	R ProtoReader `json:"-"`
}
