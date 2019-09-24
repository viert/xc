package passmgr

import (
	"fmt"
	"plugin"
)

type initFunc func() error
type acquireFunc func(string) string

const (
	initFuncName    = "Init"
	acquireFuncName = "GetPass"
)

var (
	p             *plugin.Plugin
	initialized   bool
	pluginInit    initFunc
	pluginAcquire acquireFunc
)

// Load loads a password manager library
func Load(filename string) error {
	var err error
	p, err = plugin.Open(filename)
	if err != nil {
		return err
	}

	init, err := p.Lookup(initFuncName)
	if err != nil {
		return err
	}
	pluginInit, initialized = init.(initFunc)
	if !initialized {
		return fmt.Errorf("invalid plugin `%s() error` function signature", initFuncName)
	}

	acq, err := p.Lookup(acquireFuncName)
	if err != nil {
		return err
	}
	pluginAcquire, initialized = acq.(acquireFunc)
	if !initialized {
		return fmt.Errorf("invalid plugin `%s(string) string` function signature", acquireFuncName)
	}
	return nil
}

// GetPass returns password for a given host from password manager
func GetPass(hostname string) string {
	if !initialized {
		return ""
	}
	return pluginAcquire(hostname)
}

// Ready returns a bool value indicating if the passmgr is initialized and ready to use
func Ready() bool {
	return initialized
}
