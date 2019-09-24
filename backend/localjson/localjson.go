package localjson

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/viert/xc/config"
	"github.com/viert/xc/store"
)

// Data is struct for JSON data
type Data struct {
	Hosts       []*store.Host
	Groups      []*store.Group
	Workgroups  []*store.WorkGroup
	Datacenters []*store.Datacenter
}

// LocalJSON is struct for JSON config file
type LocalJSON struct {
	filename string
	Data     Data
}

// New used for Init JSON backend
func New(cfg *config.XCConfig) (*LocalJSON, error) {
	filename, found := cfg.BackendCfg.Options["filename"]
	if !found {
		return nil, fmt.Errorf("localjson backend filename option is missing")
	}
	return &LocalJSON{filename: filename}, nil
}

// Hosts exported backend method
func (lj *LocalJSON) Hosts() []*store.Host {
	return lj.Data.Hosts
}

// Groups exported backend method
func (lj *LocalJSON) Groups() []*store.Group {
	return lj.Data.Groups
}

// WorkGroups exported backend method
func (lj *LocalJSON) WorkGroups() []*store.WorkGroup {
	return lj.Data.Workgroups
}

// Datacenters exported backend method
func (lj *LocalJSON) Datacenters() []*store.Datacenter {
	return lj.Data.Datacenters
}

// Load used for load JSON file from disk
func (lj *LocalJSON) Load() error {
	f, err := os.Open(lj.filename)
	defer f.Close()
	if err != nil {
		return err
	}

	return lj.read(f)
}

func (lj *LocalJSON) read(f *os.File) error {
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &lj.Data)
	if err != nil {
		return err
	}
	for i := range lj.Data.Hosts {
		lj.Data.Hosts[i].FQDN = lj.Data.Hosts[i].ID
	}
	for i := range lj.Data.Groups {
		lj.Data.Groups[i].Name = lj.Data.Groups[i].ID
	}
	for i := range lj.Data.Workgroups {
		lj.Data.Workgroups[i].Name = lj.Data.Workgroups[i].ID
	}
	for i := range lj.Data.Datacenters {
		lj.Data.Datacenters[i].Name = lj.Data.Datacenters[i].ID
	}
	return nil
}

// Reload implements Load call for reload file from disk
func (lj *LocalJSON) Reload() error {
	return lj.Load()
}
