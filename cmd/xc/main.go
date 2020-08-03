package main

import (
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/viert/xc/backend/conductor"
	"github.com/viert/xc/backend/inventoree"
	"github.com/viert/xc/backend/localini"

	_ "net/http/pprof"

	"github.com/viert/xc/cli"
	"github.com/viert/xc/config"
	"github.com/viert/xc/term"
)

func main() {

	go http.ListenAndServe(":5001", nil)

	var tool *cli.Cli
	var err error

	cfgFilename := path.Join(os.Getenv("HOME"), ".xc.conf")
	xccfg, err := config.Read(cfgFilename)
	if err != nil {
		term.Errorf("Error reading config: %s\n", err)
		return
	}

	switch xccfg.BackendCfg.Type {
	case config.BTInventoree:
		be, err := inventoree.New(xccfg)
		if err != nil {
			term.Errorf("Error creating inventoree backend: %s\n", err)
			return
		}

		tool, err = cli.New(xccfg, be)
		if err != nil {
			term.Errorf("%s\n", err)
			return
		}
	case config.BTIni:
		be, err := localini.New(xccfg)
		if err != nil {
			term.Errorf("Error creating local ini backend: %s\n", err)
			return
		}

		tool, err = cli.New(xccfg, be)
		if err != nil {
			term.Errorf("%s\n", err)
			return
		}

	case config.BTConductor:
		be, err := conductor.New(xccfg)
		if err != nil {
			term.Errorf("Error creating conductor backend: %s\n", err)
			return
		}

		tool, err = cli.New(xccfg, be)
		if err != nil {
			term.Errorf("%s\n", err)
			return
		}
	default:
		term.Errorf("Backend type %s is not implemented yet\n", xccfg.BackendCfg.TypeString)
		return
	}

	defer tool.Finalize()
	if len(os.Args) < 2 {
		tool.CmdLoop()
	} else {
		cmd := strings.Join(os.Args[1:], " ")
		tool.OneCmd(cmd)
	}
}
