package fcm

import (
	"encoding/json"
)

// subscriptionSync is used to sync subscriptions with other nodes
type subscriptionSync struct {
	Topic  string
	UserID string
	FCMID  string
	Remove bool
}

func (s *subscriptionSync) Encode() ([]byte, error) {
	return json.Marshal(s)
}

func (s *subscriptionSync) Decode(data []byte) (*subscriptionSync, error) {
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}
