package store

// This is a dummy implementation of the MessageStore interface.
// It is intended for testing and demo purpose, as well as dummy for services without persistance.
// TODO: implement a simple logik to preserve the last n messages
type DummyMessageStore struct {
}

func NewDummyMessageStore() *DummyMessageStore {
	return &DummyMessageStore{}
}

func (fms *DummyMessageStore) Store(partition string, msgId uint64, msg []byte) error {
	return nil
}

func (fms *DummyMessageStore) Fetch(req FetchRequest) {
}

func (fms *DummyMessageStore) partitionStore(partition string) (*MessagePartition, error) {
	return nil, nil
}
