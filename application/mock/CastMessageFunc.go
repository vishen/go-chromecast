// Code generated by mockery v2.8.0. DO NOT EDIT.

package mock

import (
	api "github.com/vishen/go-chromecast/cast/proto"

	mock "github.com/stretchr/testify/mock"
)

// CastMessageFunc is an autogenerated mock type for the CastMessageFunc type
type CastMessageFunc struct {
	mock.Mock
}

// Execute provides a mock function with given fields: _a0
func (_m *CastMessageFunc) Execute(_a0 *api.CastMessage) {
	_m.Called(_a0)
}
