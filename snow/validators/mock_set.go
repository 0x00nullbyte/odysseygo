// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/avalanchego/snow/validators (interfaces: Set)

// Package validators is a generated GoMock package.
package validators

import (
	reflect "reflect"

	ids "github.com/ava-labs/avalanchego/ids"
	bls "github.com/ava-labs/avalanchego/utils/crypto/bls"
	set "github.com/ava-labs/avalanchego/utils/set"
	gomock "github.com/golang/mock/gomock"
)

// MockSet is a mock of Set interface.
type MockSet struct {
	ctrl     *gomock.Controller
	recorder *MockSetMockRecorder
}

// MockSetMockRecorder is the mock recorder for MockSet.
type MockSetMockRecorder struct {
	mock *MockSet
}

// NewMockSet creates a new mock instance.
func NewMockSet(ctrl *gomock.Controller) *MockSet {
	mock := &MockSet{ctrl: ctrl}
	mock.recorder = &MockSetMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSet) EXPECT() *MockSetMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockSet) Add(arg0 ids.NodeID, arg1 *bls.PublicKey, arg2 ids.ID, arg3 uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Add indicates an expected call of Add.
func (mr *MockSetMockRecorder) Add(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockSet)(nil).Add), arg0, arg1, arg2, arg3)
}

// AddWeight mocks base method.
func (m *MockSet) AddWeight(arg0 ids.NodeID, arg1 uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddWeight", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddWeight indicates an expected call of AddWeight.
func (mr *MockSetMockRecorder) AddWeight(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddWeight", reflect.TypeOf((*MockSet)(nil).AddWeight), arg0, arg1)
}

// Contains mocks base method.
func (m *MockSet) Contains(arg0 ids.NodeID) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Contains", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Contains indicates an expected call of Contains.
func (mr *MockSetMockRecorder) Contains(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Contains", reflect.TypeOf((*MockSet)(nil).Contains), arg0)
}

// Get mocks base method.
func (m *MockSet) Get(arg0 ids.NodeID) (*Validator, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0)
	ret0, _ := ret[0].(*Validator)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockSetMockRecorder) Get(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockSet)(nil).Get), arg0)
}

// GetWeight mocks base method.
func (m *MockSet) GetWeight(arg0 ids.NodeID) uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetWeight", arg0)
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetWeight indicates an expected call of GetWeight.
func (mr *MockSetMockRecorder) GetWeight(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetWeight", reflect.TypeOf((*MockSet)(nil).GetWeight), arg0)
}

// Len mocks base method.
func (m *MockSet) Len() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Len")
	ret0, _ := ret[0].(int)
	return ret0
}

// Len indicates an expected call of Len.
func (mr *MockSetMockRecorder) Len() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Len", reflect.TypeOf((*MockSet)(nil).Len))
}

// List mocks base method.
func (m *MockSet) List() []*Validator {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List")
	ret0, _ := ret[0].([]*Validator)
	return ret0
}

// List indicates an expected call of List.
func (mr *MockSetMockRecorder) List() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockSet)(nil).List))
}

// PrefixedString mocks base method.
func (m *MockSet) PrefixedString(arg0 string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrefixedString", arg0)
	ret0, _ := ret[0].(string)
	return ret0
}

// PrefixedString indicates an expected call of PrefixedString.
func (mr *MockSetMockRecorder) PrefixedString(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrefixedString", reflect.TypeOf((*MockSet)(nil).PrefixedString), arg0)
}

// RegisterCallbackListener mocks base method.
func (m *MockSet) RegisterCallbackListener(arg0 SetCallbackListener) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RegisterCallbackListener", arg0)
}

// RegisterCallbackListener indicates an expected call of RegisterCallbackListener.
func (mr *MockSetMockRecorder) RegisterCallbackListener(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterCallbackListener", reflect.TypeOf((*MockSet)(nil).RegisterCallbackListener), arg0)
}

// RemoveWeight mocks base method.
func (m *MockSet) RemoveWeight(arg0 ids.NodeID, arg1 uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveWeight", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveWeight indicates an expected call of RemoveWeight.
func (mr *MockSetMockRecorder) RemoveWeight(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveWeight", reflect.TypeOf((*MockSet)(nil).RemoveWeight), arg0, arg1)
}

// Sample mocks base method.
func (m *MockSet) Sample(arg0 int) ([]ids.NodeID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sample", arg0)
	ret0, _ := ret[0].([]ids.NodeID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Sample indicates an expected call of Sample.
func (mr *MockSetMockRecorder) Sample(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sample", reflect.TypeOf((*MockSet)(nil).Sample), arg0)
}

// String mocks base method.
func (m *MockSet) String() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "String")
	ret0, _ := ret[0].(string)
	return ret0
}

// String indicates an expected call of String.
func (mr *MockSetMockRecorder) String() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "String", reflect.TypeOf((*MockSet)(nil).String))
}

// SubsetWeight mocks base method.
func (m *MockSet) SubsetWeight(arg0 set.Set[ids.NodeID]) uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubsetWeight", arg0)
	ret0, _ := ret[0].(uint64)
	return ret0
}

// SubsetWeight indicates an expected call of SubsetWeight.
func (mr *MockSetMockRecorder) SubsetWeight(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubsetWeight", reflect.TypeOf((*MockSet)(nil).SubsetWeight), arg0)
}

// Weight mocks base method.
func (m *MockSet) Weight() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Weight")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Weight indicates an expected call of Weight.
func (mr *MockSetMockRecorder) Weight() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Weight", reflect.TypeOf((*MockSet)(nil).Weight))
}
