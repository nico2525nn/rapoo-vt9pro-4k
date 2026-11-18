//go:build !windows && !linux

package hid

type unsupportedProvider struct{}

func newNativeProvider() Provider {
	return &unsupportedProvider{}
}

func (p *unsupportedProvider) Enumerate(_ Filter) ([]DeviceInfo, error) {
	return nil, ErrUnsupported
}

func (p *unsupportedProvider) Open(_ DeviceInfo, _ bool) (Device, error) {
	return nil, ErrUnsupported
}
