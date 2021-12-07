//go:build windows
// +build windows

package edgevpn

import "github.com/songgao/water"

func (e *EdgeVPN) prepareInterface() error {

	return nil
}

func (e *EdgeVPN) createInterface() (*water.Interface, error) {
	config := water.Config{
		DeviceType: e.config.DeviceType,
	}

	return water.New(config)
}
