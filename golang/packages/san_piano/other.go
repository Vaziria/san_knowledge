//go:build !windows

package san_piano

type platformInput struct{}

func devices() ([]Device, error) { return nil, ErrUnsupported }

func open(Device) (*Input, error) { return nil, ErrUnsupported }

func (in *Input) close() error { return nil }
