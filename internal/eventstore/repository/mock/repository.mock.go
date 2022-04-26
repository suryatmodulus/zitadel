// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/zitadel/zitadel/internal/eventstore/repository (interfaces: Repository)

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	repository "github.com/zitadel/zitadel/internal/eventstore/repository"
	gomock "github.com/golang/mock/gomock"
)

// MockRepository is a mock of Repository interface.
type MockRepository struct {
	ctrl     *gomock.Controller
	recorder *MockRepositoryMockRecorder
}

// MockRepositoryMockRecorder is the mock recorder for MockRepository.
type MockRepositoryMockRecorder struct {
	mock *MockRepository
}

// NewMockRepository creates a new mock instance.
func NewMockRepository(ctrl *gomock.Controller) *MockRepository {
	mock := &MockRepository{ctrl: ctrl}
	mock.recorder = &MockRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRepository) EXPECT() *MockRepositoryMockRecorder {
	return m.recorder
}

// Filter mocks base method.
func (m *MockRepository) Filter(arg0 context.Context, arg1 *repository.SearchQuery) ([]*repository.Event, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filter", arg0, arg1)
	ret0, _ := ret[0].([]*repository.Event)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Filter indicates an expected call of Filter.
func (mr *MockRepositoryMockRecorder) Filter(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filter", reflect.TypeOf((*MockRepository)(nil).Filter), arg0, arg1)
}

// Health mocks base method.
func (m *MockRepository) Health(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Health", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Health indicates an expected call of Health.
func (mr *MockRepositoryMockRecorder) Health(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Health", reflect.TypeOf((*MockRepository)(nil).Health), arg0)
}

// LatestSequence mocks base method.
func (m *MockRepository) LatestSequence(arg0 context.Context, arg1 *repository.SearchQuery) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LatestSequence", arg0, arg1)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LatestSequence indicates an expected call of LatestSequence.
func (mr *MockRepositoryMockRecorder) LatestSequence(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LatestSequence", reflect.TypeOf((*MockRepository)(nil).LatestSequence), arg0, arg1)
}

// Push mocks base method.
func (m *MockRepository) Push(arg0 context.Context, arg1 []*repository.Event, arg2 ...*repository.UniqueConstraint) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Push", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Push indicates an expected call of Push.
func (mr *MockRepositoryMockRecorder) Push(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Push", reflect.TypeOf((*MockRepository)(nil).Push), varargs...)
}

// Step20 mocks base method.
func (m *MockRepository) Step20(arg0 context.Context, arg1 uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Step20", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Step20 indicates an expected call of Step20.
func (mr *MockRepositoryMockRecorder) Step20(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Step20", reflect.TypeOf((*MockRepository)(nil).Step20), arg0, arg1)
}
