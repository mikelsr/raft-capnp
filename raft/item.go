package raft

import (
	"encoding/json"

	"capnproto.org/go/capnp/v3"
	"github.com/mikelsr/raft-capnp/proto/api"
)

type Item struct {
	Key   []byte `json:"key,omitempty"`
	Value []byte `json:"value,omitempty"`
}

func (i Item) Marshal() ([]byte, error) {
	return json.Marshal(i)
}

func (i *Item) Unmarshal(data []byte) error {
	return json.Unmarshal(data, i)
}

func ItemFromApi(i api.Item) (Item, error) {
	var item Item

	k, err := i.Key()
	if err != nil {
		return item, err
	}
	kBuf := make([]byte, len(k))
	copy(kBuf, k)
	item.Key = kBuf

	v, err := i.Value()
	if err != nil {
		return item, err
	}
	vBuf := make([]byte, len(v))
	copy(vBuf, v)
	item.Value = v

	return item, nil
}

func ItemToApi(i Item) (api.Item, error) {
	var item api.Item
	var err error

	_, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return item, err
	}

	item, err = api.NewItem(seg)
	if err != nil {
		return item, err
	}

	if err = item.SetKey(i.Key); err != nil {
		return item, err
	}

	err = item.SetValue(i.Value)

	return item, err
}
