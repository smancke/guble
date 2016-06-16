package cluster

import (
	"bytes"
	"strconv"
)

type NextID int

func (id *NextID) Bytes() []byte {
	buff := &bytes.Buffer{}

	buff.WriteString(strconv.Itoa(int(*id)))
	return buff.Bytes()
}

func DecodeNextID(payload []byte) (*NextID, error) {
	i, err := strconv.Atoi(string(payload))
	if err != nil {
		return nil, err
	}

	tt := NextID(int(i))

	return &tt, nil
}
