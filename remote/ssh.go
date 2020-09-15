package remote

import (
	"fmt"
)

var (
	sshOptions = map[string]string{
		"PasswordAuthentication": "no",
		"PubkeyAuthentication":   "yes",
		"StrictHostKeyChecking":  "no",
		"TCPKeepAlive":           "yes",
		"ServerAliveCountMax":    "12",
		"ServerAliveInterval":    "5",
	}
)

// ApplyConfiguredOptions merges default options and configured ones
func ApplyConfiguredOptions(cfgOptions map[string]string) {
	for k, v := range cfgOptions {
		sshOptions[k] = v
	}
}

func sshOpts() (params []string) {
	params = make([]string, 0)
	for opt, value := range sshOptions {
		option := fmt.Sprintf("%s=%s", opt, value)
		params = append(params, "-o", option)
	}
	return
}
