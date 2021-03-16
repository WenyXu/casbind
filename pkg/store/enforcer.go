/*
Copyright The casbind Authors.
@Date: 2021/03/16 18:41
*/

package store

import (
	"context"
	"encoding/json"

	"github.com/WenyXu/casbind/proto/command"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/raft"
)

// CreateNamespace
func (s *Store) CreateNamespace(ctx context.Context, ns string) error {
	cmd, err := proto.Marshal(&command.Command{
		Type:       command.Type_COMMAND_TYPE_CREATE_NS,
		Ns:         ns,
		Payload:    nil,
		Md:         nil,
		Compressed: false,
	})
	if err != nil {
		return err
	}
	f := s.raft.Apply(cmd, s.ApplyTimeout)
	if e := f.(raft.Future); e.Error() != nil {
		if e.Error() == raft.ErrNotLeader {
			return ErrNotLeader
		}
		return e.Error()
	}
	r := f.Response().(*FSMResponse)
	return r.error
}

// SetModelFromString
func (s *Store) SetModelFromString(ctx context.Context, ns string, text string) error {
	payload, err := proto.Marshal(&command.SetModelFromString{
		Text: text,
	})
	if err != nil {
		return err
	}

	cmd, err := proto.Marshal(&command.Command{
		Type:       command.Type_COMMAND_TYPE_SET_MODEL,
		Ns:         ns,
		Payload:    payload,
		Md:         nil,
		Compressed: false,
	})
	if err != nil {
		return err
	}
	f := s.raft.Apply(cmd, s.ApplyTimeout)
	if e := f.(raft.Future); e.Error() != nil {
		if e.Error() == raft.ErrNotLeader {
			return ErrNotLeader
		}
		return e.Error()
	}
	r := f.Response().(*FSMResponse)
	return r.error
}

// Enforce
func (s *Store) Enforce(ctx context.Context, ns string, level command.EnforcePayload_Level, freshness int64, param ...interface{}) (bool, error) {
	var B [][]byte
	for _, p := range param {
		b, err := json.Marshal(p)
		if err != nil {
			return false, err
		}
		B = append(B, b)
	}

	payload, err := proto.Marshal(&command.EnforcePayload{
		B:         B,
		Level:     level,
		Freshness: freshness,
	})
	if err != nil {
		return false, err
	}

	cmd, err := proto.Marshal(&command.Command{
		Type:       command.Type_COMMAND_TYPE_ENFORCE_REQUEST,
		Ns:         ns,
		Payload:    payload,
		Md:         nil,
		Compressed: false,
	})
	if err != nil {
		return false, err
	}
	f := s.raft.Apply(cmd, s.ApplyTimeout)
	if e := f.(raft.Future); e.Error() != nil {
		if e.Error() == raft.ErrNotLeader {
			return false, err
		}
		return false, e.Error()
	}
	r := f.Response().(*FSMEnforceResponse)
	return r.ok, r.error
}

// SetMetadata adds the metadata md to any existing metadata for
// this node.
func (s *Store) SetMetadata(md map[string]string) error {
	return s.setMetadata(s.raftID, md)
}

// setMetadata adds the metadata md to any existing metadata for
// the given node ID.
func (s *Store) setMetadata(id string, md map[string]string) error {
	// Check local data first.
	if func() bool {
		s.metaMu.RLock()
		defer s.metaMu.RUnlock()
		if _, ok := s.meta[id]; ok {
			for k, v := range md {
				if s.meta[id][k] != v {
					return false
				}
			}
			return true
		}
		return false
	}() {
		// Local data is same as data being pushed in,
		// nothing to do.
		return nil
	}

	ms := &command.MetadataSet{
		RaftId: id,
		Data:   md,
	}
	bms, err := proto.Marshal(ms)
	if err != nil {
		return err
	}

	c := &command.Command{
		Type:    command.Type_COMMAND_TYPE_METADATA_SET,
		Payload: bms,
	}
	bc, err := proto.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(bc, s.ApplyTimeout)
	if e := f.(raft.Future); e.Error() != nil {
		if e.Error() == raft.ErrNotLeader {
			return ErrNotLeader
		}
		return e.Error()
	}

	return nil
}
