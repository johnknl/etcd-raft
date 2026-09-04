package tooling

import (
	"fmt"
	"go/format"
	"path/filepath"
)

const wrappersSrc = `// Copyright 2026 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raftpb

import "reflect"

func NewEntry(term, index uint64, typ EntryType, data []byte) *Entry {
	return new(Entry).SetTerm(term).SetIndex(index).SetType(typ).SetData(data)
}

func NewEmptyEntry() *Entry           { return new(Entry) }
func NewEntryData(data []byte) *Entry { return new(Entry).SetData(data) }
func NewEntryRef(term, index interface{}) *Entry {
	return new(Entry).SetTermPtr(term).SetIndexPtr(index)
}

func NewMessage(typ MessageType, from, to interface{}) *Message {
	return new(Message).SetType(typ).SetFromPtr(from).SetToPtr(to)
}

func NewEmptyMessage() *Message { return new(Message) }

func NewHardState(term, vote, commit interface{}) *HardState {
	return new(HardState).SetTermPtr(term).SetVotePtr(vote).SetCommitPtr(commit)
}

func NewEmptyHardState() *HardState { return new(HardState) }

func NewConfChange(typ ConfChangeType, nodeID interface{}, context []byte) *ConfChange {
	return new(ConfChange).SetType(typ).SetNodeIDPtr(nodeID).SetContext(context)
}

func NewEmptyConfChange() *ConfChange { return new(ConfChange) }

func NewConfChangeSingle(typ ConfChangeType, nodeID interface{}) *ConfChangeSingle {
	return new(ConfChangeSingle).SetType(typ).SetNodeIDPtr(nodeID)
}

func NewEmptyConfChangeSingle() *ConfChangeSingle { return new(ConfChangeSingle) }

func NewConfChangeV2(transition ConfChangeTransition, changes []*ConfChangeSingle, context []byte) *ConfChangeV2 {
	return new(ConfChangeV2).SetTransition(transition).SetChanges(changes).SetContext(context)
}

func NewEmptyConfChangeV2() *ConfChangeV2 { return new(ConfChangeV2) }

func NewConfState(voters, learners, votersOutgoing, learnersNext []uint64, autoLeave bool) *ConfState {
	cs := &ConfState{
		Voters:         append([]uint64(nil), voters...),
		Learners:       append([]uint64(nil), learners...),
		VotersOutgoing: append([]uint64(nil), votersOutgoing...),
		LearnersNext:   append([]uint64(nil), learnersNext...),
	}
	return cs.SetAutoLeave(autoLeave)
}

func NewEmptyConfState() *ConfState { return new(ConfState) }

func NewSnapshotMetadata(index, term interface{}, cs *ConfState) *SnapshotMetadata {
	return new(SnapshotMetadata).SetConfState(cs).SetIndexPtr(index).SetTermPtr(term)
}

func NewEmptySnapshotMetadata() *SnapshotMetadata { return new(SnapshotMetadata) }

func NewSnapshot(metadata *SnapshotMetadata, data []byte) *Snapshot {
	return new(Snapshot).SetMetadata(metadata).SetData(data)
}

func NewEmptySnapshot() *Snapshot { return new(Snapshot) }

func uint64Ptr(v interface{}) *uint64 {
	switch x := v.(type) {
	case uint64:
		return &x
	case *uint64:
		return x
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		panic("uint64Ptr: invalid value")
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := rv.Convert(reflect.TypeOf(uint64(0))).Interface().(uint64)
		return &u
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		u := uint64(rv.Int())
		return &u
	default:
		panic("uint64Ptr: unsupported type")
	}
}

func boolPtr(v interface{}) *bool {
	switch x := v.(type) {
	case bool:
		return &x
	case *bool:
		return x
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		panic("boolPtr: invalid value")
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Bool {
		b := rv.Bool()
		return &b
	}
	panic("boolPtr: unsupported type")
}

func (e *Entry) SetTerm(term uint64) *Entry   { e.Term = &term; return e }
func (e *Entry) SetIndex(index uint64) *Entry { e.Index = &index; return e }
func (e *Entry) SetType(typ EntryType) *Entry { e.Type = &typ; return e }
func (e *Entry) SetData(data []byte) *Entry   { e.Data = data; return e }
func (e *Entry) SetTermPtr(term interface{}) *Entry {
	e.Term = uint64Ptr(term)
	return e
}
func (e *Entry) SetIndexPtr(index interface{}) *Entry {
	e.Index = uint64Ptr(index)
	return e
}

func (m *Message) SetType(typ MessageType) *Message           { m.Type = &typ; return m }
func (m *Message) SetTo(to uint64) *Message                   { m.To = &to; return m }
func (m *Message) SetFrom(from uint64) *Message               { m.From = &from; return m }
func (m *Message) SetTerm(term uint64) *Message               { m.Term = &term; return m }
func (m *Message) SetLogTerm(logTerm uint64) *Message         { m.LogTerm = &logTerm; return m }
func (m *Message) SetIndex(index uint64) *Message             { m.Index = &index; return m }
func (m *Message) SetCommit(commit uint64) *Message           { m.Commit = &commit; return m }
func (m *Message) SetVote(vote uint64) *Message               { m.Vote = &vote; return m }
func (m *Message) SetSnapshot(s *Snapshot) *Message           { m.Snapshot = s; return m }
func (m *Message) SetReject(reject bool) *Message             { m.Reject = &reject; return m }
func (m *Message) SetRejectHint(rejectHint uint64) *Message   { m.RejectHint = &rejectHint; return m }
func (m *Message) SetContext(context []byte) *Message         { m.Context = context; return m }
func (m *Message) SetEntries(entries []*Entry) *Message       { m.Entries = entries; return m }
func (m *Message) SetResponses(responses []*Message) *Message { m.Responses = responses; return m }
func (m *Message) SetToPtr(to interface{}) *Message {
	m.To = uint64Ptr(to)
	return m
}
func (m *Message) SetFromPtr(from interface{}) *Message {
	m.From = uint64Ptr(from)
	return m
}
func (m *Message) SetTermPtr(term interface{}) *Message {
	m.Term = uint64Ptr(term)
	return m
}
func (m *Message) SetLogTermPtr(logTerm interface{}) *Message {
	m.LogTerm = uint64Ptr(logTerm)
	return m
}
func (m *Message) SetIndexPtr(index interface{}) *Message {
	m.Index = uint64Ptr(index)
	return m
}
func (m *Message) SetCommitPtr(commit interface{}) *Message {
	m.Commit = uint64Ptr(commit)
	return m
}
func (m *Message) SetVotePtr(vote interface{}) *Message {
	m.Vote = uint64Ptr(vote)
	return m
}
func (m *Message) SetRejectPtr(reject interface{}) *Message {
	m.Reject = boolPtr(reject)
	return m
}
func (m *Message) SetRejectHintPtr(rejectHint interface{}) *Message {
	m.RejectHint = uint64Ptr(rejectHint)
	return m
}

func (h *HardState) SetTerm(term uint64) *HardState     { h.Term = &term; return h }
func (h *HardState) SetVote(vote uint64) *HardState     { h.Vote = &vote; return h }
func (h *HardState) SetCommit(commit uint64) *HardState { h.Commit = &commit; return h }
func (h *HardState) SetTermPtr(term interface{}) *HardState {
	h.Term = uint64Ptr(term)
	return h
}
func (h *HardState) SetVotePtr(vote interface{}) *HardState {
	h.Vote = uint64Ptr(vote)
	return h
}
func (h *HardState) SetCommitPtr(commit interface{}) *HardState {
	h.Commit = uint64Ptr(commit)
	return h
}

func (s *SnapshotMetadata) SetConfState(cs *ConfState) *SnapshotMetadata { s.ConfState = cs; return s }
func (s *SnapshotMetadata) SetIndex(index uint64) *SnapshotMetadata      { s.Index = &index; return s }
func (s *SnapshotMetadata) SetTerm(term uint64) *SnapshotMetadata        { s.Term = &term; return s }
func (s *SnapshotMetadata) SetIndexPtr(index interface{}) *SnapshotMetadata {
	s.Index = uint64Ptr(index)
	return s
}
func (s *SnapshotMetadata) SetTermPtr(term interface{}) *SnapshotMetadata {
	s.Term = uint64Ptr(term)
	return s
}

func (s *Snapshot) SetMetadata(metadata *SnapshotMetadata) *Snapshot { s.Metadata = metadata; return s }
func (s *Snapshot) SetData(data []byte) *Snapshot                    { s.Data = data; return s }
func (s *Snapshot) DataRef() *[]byte                                 { return &s.Data }

func (cs *ConfState) SetAutoLeave(autoLeave bool) *ConfState { cs.AutoLeave = &autoLeave; return cs }
func (cs *ConfState) SetAutoLeavePtr(autoLeave interface{}) *ConfState {
	cs.AutoLeave = boolPtr(autoLeave)
	return cs
}
func (cs *ConfState) SetVoters(voters []uint64) *ConfState {
	cs.Voters = append(cs.Voters[:0], voters...)
	return cs
}
func (cs *ConfState) SetLearners(learners []uint64) *ConfState {
	cs.Learners = append(cs.Learners[:0], learners...)
	return cs
}
func (cs *ConfState) SetVotersOutgoing(votersOutgoing []uint64) *ConfState {
	cs.VotersOutgoing = append(cs.VotersOutgoing[:0], votersOutgoing...)
	return cs
}
func (cs *ConfState) SetLearnersNext(learnersNext []uint64) *ConfState {
	cs.LearnersNext = append(cs.LearnersNext[:0], learnersNext...)
	return cs
}

func (c *ConfChange) SetType(typ ConfChangeType) *ConfChange { c.Type = &typ; return c }
func (c *ConfChange) SetNodeID(nodeID uint64) *ConfChange    { c.NodeId = &nodeID; return c }
func (c *ConfChange) SetContext(context []byte) *ConfChange  { c.Context = context; return c }
func (c *ConfChange) SetID(id uint64) *ConfChange            { c.Id = &id; return c }
func (c *ConfChange) SetNodeIDPtr(nodeID interface{}) *ConfChange {
	c.NodeId = uint64Ptr(nodeID)
	return c
}
func (c *ConfChange) SetIDPtr(id interface{}) *ConfChange {
	c.Id = uint64Ptr(id)
	return c
}

func (c *ConfChangeSingle) SetType(typ ConfChangeType) *ConfChangeSingle { c.Type = &typ; return c }
func (c *ConfChangeSingle) SetNodeID(nodeID uint64) *ConfChangeSingle    { c.NodeId = &nodeID; return c }
func (c *ConfChangeSingle) SetNodeIDPtr(nodeID interface{}) *ConfChangeSingle {
	c.NodeId = uint64Ptr(nodeID)
	return c
}

func (c *ConfChangeV2) SetTransition(transition ConfChangeTransition) *ConfChangeV2 {
	c.Transition = &transition
	return c
}
func (c *ConfChangeV2) SetChanges(changes []*ConfChangeSingle) *ConfChangeV2 {
	c.Changes = changes
	return c
}
func (c *ConfChangeV2) SetContext(context []byte) *ConfChangeV2 { c.Context = context; return c }
`

func EnsureWrappers(repoRoot string) error {
	b, err := format.Source([]byte(wrappersSrc))
	if err != nil {
		return fmt.Errorf("format wrappers.go: %w", err)
	}
	_, err = WriteIfChanged(filepath.Join(repoRoot, "raftpb", "wrappers.go"), b)
	return err
}
