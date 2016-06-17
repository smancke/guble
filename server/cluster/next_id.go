package cluster

import (
	"bytes"
	"strconv"
)

type NextID uint64

func (id *NextID) Bytes() []byte {
	buff := &bytes.Buffer{}
	buff.WriteString(strconv.FormatUint(uint64(*id), 10))
	return buff.Bytes()
}

func decodeNextID(payload []byte) (*NextID, error) {
	i, err := strconv.ParseUint(string(payload), 10, 64)
	if err != nil {
		return nil, err
	}
	tt := NextID(int(i))
	return &tt, nil
}
