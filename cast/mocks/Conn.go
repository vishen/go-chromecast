// Code generated by mockery v2.49.1. DO NOT EDIT.

package mocks

import (
	cast "github.com/vishen/go-chromecast/cast"
	api "github.com/vishen/go-chromecast/cast/proto"

	mock "github.com/stretchr/testify/mock"
)

// Conn is an autogenerated mock type for the Conn type
type Conn struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Conn) Close() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// LocalAddr provides a mock function with given fields:
func (_m *Conn) LocalAddr() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for LocalAddr")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MsgChan provides a mock function with given fields:
func (_m *Conn) MsgChan() chan *api.CastMessage {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for MsgChan")
	}

	var r0 chan *api.CastMessage
	if rf, ok := ret.Get(0).(func() chan *api.CastMessage); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan *api.CastMessage)
		}
	}

	return r0
}

// RemoteAddr provides a mock function with given fields:
func (_m *Conn) RemoteAddr() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for RemoteAddr")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemotePort provides a mock function with given fields:
func (_m *Conn) RemotePort() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for RemotePort")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Send provides a mock function with given fields: requestID, payload, sourceID, destinationID, namespace
func (_m *Conn) Send(requestID int, payload cast.Payload, sourceID string, destinationID string, namespace string) error {
	ret := _m.Called(requestID, payload, sourceID, destinationID, namespace)

	if len(ret) == 0 {
		panic("no return value specified for Send")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(int, cast.Payload, string, string, string) error); ok {
		r0 = rf(requestID, payload, sourceID, destinationID, namespace)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetDebug provides a mock function with given fields: debug
func (_m *Conn) SetDebug(debug bool) {
	_m.Called(debug)
}

// Start provides a mock function with given fields: addr, port
func (_m *Conn) Start(addr string, port int) error {
	ret := _m.Called(addr, port)

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, int) error); ok {
		r0 = rf(addr, port)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewConn creates a new instance of Conn. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewConn(t interface {
	mock.TestingT
	Cleanup(func())
}) *Conn {
	mock := &Conn{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
