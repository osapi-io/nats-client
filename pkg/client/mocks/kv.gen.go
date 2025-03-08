// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/nats-io/nats.go (interfaces: KeyValue)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	nats "github.com/nats-io/nats.go"
)

// MockKeyValue is a mock of KeyValue interface.
type MockKeyValue struct {
	ctrl     *gomock.Controller
	recorder *MockKeyValueMockRecorder
}

// MockKeyValueMockRecorder is the mock recorder for MockKeyValue.
type MockKeyValueMockRecorder struct {
	mock *MockKeyValue
}

// NewMockKeyValue creates a new mock instance.
func NewMockKeyValue(ctrl *gomock.Controller) *MockKeyValue {
	mock := &MockKeyValue{ctrl: ctrl}
	mock.recorder = &MockKeyValueMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockKeyValue) EXPECT() *MockKeyValueMockRecorder {
	return m.recorder
}

// Bucket mocks base method.
func (m *MockKeyValue) Bucket() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Bucket")
	ret0, _ := ret[0].(string)
	return ret0
}

// Bucket indicates an expected call of Bucket.
func (mr *MockKeyValueMockRecorder) Bucket() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Bucket", reflect.TypeOf((*MockKeyValue)(nil).Bucket))
}

// Create mocks base method.
func (m *MockKeyValue) Create(arg0 string, arg1 []byte) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", arg0, arg1)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockKeyValueMockRecorder) Create(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockKeyValue)(nil).Create), arg0, arg1)
}

// Delete mocks base method.
func (m *MockKeyValue) Delete(arg0 string, arg1 ...nats.DeleteOpt) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Delete", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockKeyValueMockRecorder) Delete(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKeyValue)(nil).Delete), varargs...)
}

// Get mocks base method.
func (m *MockKeyValue) Get(arg0 string) (nats.KeyValueEntry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0)
	ret0, _ := ret[0].(nats.KeyValueEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockKeyValueMockRecorder) Get(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockKeyValue)(nil).Get), arg0)
}

// GetRevision mocks base method.
func (m *MockKeyValue) GetRevision(arg0 string, arg1 uint64) (nats.KeyValueEntry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRevision", arg0, arg1)
	ret0, _ := ret[0].(nats.KeyValueEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRevision indicates an expected call of GetRevision.
func (mr *MockKeyValueMockRecorder) GetRevision(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRevision", reflect.TypeOf((*MockKeyValue)(nil).GetRevision), arg0, arg1)
}

// History mocks base method.
func (m *MockKeyValue) History(arg0 string, arg1 ...nats.WatchOpt) ([]nats.KeyValueEntry, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "History", varargs...)
	ret0, _ := ret[0].([]nats.KeyValueEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// History indicates an expected call of History.
func (mr *MockKeyValueMockRecorder) History(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "History", reflect.TypeOf((*MockKeyValue)(nil).History), varargs...)
}

// Keys mocks base method.
func (m *MockKeyValue) Keys(arg0 ...nats.WatchOpt) ([]string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Keys", varargs...)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Keys indicates an expected call of Keys.
func (mr *MockKeyValueMockRecorder) Keys(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Keys", reflect.TypeOf((*MockKeyValue)(nil).Keys), arg0...)
}

// ListKeys mocks base method.
func (m *MockKeyValue) ListKeys(arg0 ...nats.WatchOpt) (nats.KeyLister, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListKeys", varargs...)
	ret0, _ := ret[0].(nats.KeyLister)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListKeys indicates an expected call of ListKeys.
func (mr *MockKeyValueMockRecorder) ListKeys(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListKeys", reflect.TypeOf((*MockKeyValue)(nil).ListKeys), arg0...)
}

// Purge mocks base method.
func (m *MockKeyValue) Purge(arg0 string, arg1 ...nats.DeleteOpt) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Purge", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Purge indicates an expected call of Purge.
func (mr *MockKeyValueMockRecorder) Purge(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Purge", reflect.TypeOf((*MockKeyValue)(nil).Purge), varargs...)
}

// PurgeDeletes mocks base method.
func (m *MockKeyValue) PurgeDeletes(arg0 ...nats.PurgeOpt) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "PurgeDeletes", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// PurgeDeletes indicates an expected call of PurgeDeletes.
func (mr *MockKeyValueMockRecorder) PurgeDeletes(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PurgeDeletes", reflect.TypeOf((*MockKeyValue)(nil).PurgeDeletes), arg0...)
}

// Put mocks base method.
func (m *MockKeyValue) Put(arg0 string, arg1 []byte) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Put indicates an expected call of Put.
func (mr *MockKeyValueMockRecorder) Put(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKeyValue)(nil).Put), arg0, arg1)
}

// PutString mocks base method.
func (m *MockKeyValue) PutString(arg0, arg1 string) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PutString", arg0, arg1)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PutString indicates an expected call of PutString.
func (mr *MockKeyValueMockRecorder) PutString(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutString", reflect.TypeOf((*MockKeyValue)(nil).PutString), arg0, arg1)
}

// Status mocks base method.
func (m *MockKeyValue) Status() (nats.KeyValueStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status")
	ret0, _ := ret[0].(nats.KeyValueStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Status indicates an expected call of Status.
func (mr *MockKeyValueMockRecorder) Status() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*MockKeyValue)(nil).Status))
}

// Update mocks base method.
func (m *MockKeyValue) Update(arg0 string, arg1 []byte, arg2 uint64) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", arg0, arg1, arg2)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockKeyValueMockRecorder) Update(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockKeyValue)(nil).Update), arg0, arg1, arg2)
}

// Watch mocks base method.
func (m *MockKeyValue) Watch(arg0 string, arg1 ...nats.WatchOpt) (nats.KeyWatcher, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Watch", varargs...)
	ret0, _ := ret[0].(nats.KeyWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Watch indicates an expected call of Watch.
func (mr *MockKeyValueMockRecorder) Watch(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Watch", reflect.TypeOf((*MockKeyValue)(nil).Watch), varargs...)
}

// WatchAll mocks base method.
func (m *MockKeyValue) WatchAll(arg0 ...nats.WatchOpt) (nats.KeyWatcher, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "WatchAll", varargs...)
	ret0, _ := ret[0].(nats.KeyWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchAll indicates an expected call of WatchAll.
func (mr *MockKeyValueMockRecorder) WatchAll(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchAll", reflect.TypeOf((*MockKeyValue)(nil).WatchAll), arg0...)
}

// WatchFiltered mocks base method.
func (m *MockKeyValue) WatchFiltered(arg0 []string, arg1 ...nats.WatchOpt) (nats.KeyWatcher, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "WatchFiltered", varargs...)
	ret0, _ := ret[0].(nats.KeyWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchFiltered indicates an expected call of WatchFiltered.
func (mr *MockKeyValueMockRecorder) WatchFiltered(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchFiltered", reflect.TypeOf((*MockKeyValue)(nil).WatchFiltered), varargs...)
}
