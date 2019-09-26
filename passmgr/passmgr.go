package passmgr

import (
	"fmt"
	"plugin"

	"github.com/viert/xc/log"
	"github.com/viert/xc/term"
)

const (
	initFuncName    = "Init"
	acquireFuncName = "GetPass"
)

var (
	p               *plugin.Plugin
	initialFilename string
	initialOptions  map[string]string
	initialized     bool
	pluginInit      func(map[string]string, func(string, ...interface{})) error
	pluginAcquire   func(string) string
)

// Load loads a password manager library
func Load(filename string, options map[string]string) error {
	var err error

	initialFilename = filename
	initialOptions = options

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

// Reload reloads previously initialized plugin
func Reload() error {
	term.Warnf("Reloading password manager from %s\n", initialFilename)
	return Load(initialFilename, initialOptions)
}

// PrintDebug proxies a call to plugin's PrintDebug function if it exists
func PrintDebug() {
	if !initialized {
		term.Errorf("password manager is not initialized\n")
		return
	}

	printsym, err := p.Lookup("PrintDebug")
	if err != nil {
		term.Errorf("the password manager doesn't have PrintDebug() handler\n")
		return
	}

	printfunc, ok := printsym.(func())
	if !ok {
		term.Errorf("the passwordd manager PrintDebug() handler has invalid signature (must be func())\n")
		return
	}
	term.Warnf("running password manager PrintDebug()\n")
	printfunc()
	fmt.Println()
}
