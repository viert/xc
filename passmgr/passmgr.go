package passmgr

import (
	"fmt"
	"plugin"

	"github.com/viert/xc/log"
)

const (
	initFuncName    = "Init"
	acquireFuncName = "GetPass"
)

var (
	p             *plugin.Plugin
	initialized   bool
	pluginInit    func(map[string]string, func(string, ...interface{})) error
	pluginAcquire func(string) string
)

// Load loads a password manager library
func Load(filename string, options map[string]string) error {
	var err error
	p, err = plugin.Open(filename)
	if err != nil {
		return err
	}

	init, err := p.Lookup(initFuncName)
	if err != nil {
		return err
	}
	pluginInit, initialized = init.(func(map[string]string, func(string, ...interface{})) error)
	if !initialized {
		return fmt.Errorf("invalid plugin `%s() error` function signature", initFuncName)
	}

	acq, err := p.Lookup(acquireFuncName)
	if err != nil {
		return err
	}
	pluginAcquire, initialized = acq.(func(string) string)
	if !initialized {
		return fmt.Errorf("invalid plugin `%s(string) string` function signature", acquireFuncName)
	}

	err = pluginInit(options, log.Debugf)
	initialized = err == nil

	return err
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
