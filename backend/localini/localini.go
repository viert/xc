package localini

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/viert/xc/config"
	"github.com/viert/xc/store"
)

type parseSection int

const (
	sectionWorkgroups parseSection = iota
	sectionDatacenters
	sectionGroups
	sectionHosts
	sectionNone
)

var (
	lineCount = 0
)

// LocalIni backend loads hosts data from ini file
type LocalIni struct {
	filename    string
	hosts       []*store.Host
	groups      []*store.Group
	workgroups  []*store.WorkGroup
	datacenters []*store.Datacenter
}

// New creates a new LocalIni backend
func New(cfg *config.XCConfig) (*LocalIni, error) {
	filename, found := cfg.BackendCfg.Options["filename"]
	if !found {
		return nil, fmt.Errorf("localini backend filename option is missing")
	}
	return &LocalIni{filename: filename}, nil
}

// Hosts exported backend method
func (li *LocalIni) Hosts() []*store.Host {
	return li.hosts
}

// Groups exported backend method
func (li *LocalIni) Groups() []*store.Group {
	return li.groups
}

// WorkGroups exported backend method
func (li *LocalIni) WorkGroups() []*store.WorkGroup {
	return li.workgroups
}

// Datacenters exported backend method
func (li *LocalIni) Datacenters() []*store.Datacenter {
	return li.datacenters
}

// Load loads the data from file
func (li *LocalIni) Load() error {
	f, err := os.Open(li.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return li.read(f)
}

// Reload force reloads data from file
func (li *LocalIni) Reload() error {
	return li.Load()
}

func (li *LocalIni) read(f *os.File) error {
	var line string

	li.datacenters = make([]*store.Datacenter, 0)
	li.hosts = make([]*store.Host, 0)
	li.groups = make([]*store.Group, 0)
	li.workgroups = make([]*store.WorkGroup, 0)

	lineCount = 0
	section := sectionNone
	scan := bufio.NewScanner(f)

	for scan.Scan() {
		lineCount++
		line = strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch line {
		case "[workgroups]":
			section = sectionWorkgroups
		case "[groups]":
			section = sectionGroups
		case "[datacenters]":
			section = sectionDatacenters
		case "[hosts]":
			section = sectionHosts
		default:
			var err error
			switch section {
			case sectionNone:
				err = fmt.Errorf("Unexpected line #%d outside sections: %s", lineCount, line)
			case sectionWorkgroups:
				err = li.addWorkgroup(line)
			case sectionGroups:
				err = li.addGroup(line)
			case sectionDatacenters:
				err = li.addDatacenter(line)
			case sectionHosts:
				err = li.addHost(line)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func parseLine(line string) (map[string]string, error) {
	data := make(map[string]string)
	tokens := strings.Split(line, " ")
	if len(tokens) < 1 {
		return nil, fmt.Errorf("Malformed line, can't read workgroup name at line %d: %s", lineCount, line)
	}
	data["id"] = tokens[0]
	data["name"] = tokens[0]
	tokens = tokens[1:]
	for _, token := range tokens {
		kv := strings.Split(token, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("Invalid token \"%s\", expected key=value format at line %d", token, lineCount)
		}
		data[kv[0]] = kv[1]
	}

	return data, nil
}

func (li *LocalIni) addWorkgroup(line string) error {
	data, err := parseLine(line)
	if err != nil {
		return err
	}
	wg := new(store.WorkGroup)
	for key, value := range data {
		switch key {
		case "id":
			wg.ID = value
		case "name":
			wg.Name = value
		case "description":
			wg.Description = value
		default:
			return fmt.Errorf("Invalid token %s at line %d: %s", key, lineCount, line)
		}
	}
	li.workgroups = append(li.workgroups, wg)
	return nil
}

func (li *LocalIni) addDatacenter(line string) error {
	data, err := parseLine(line)
	if err != nil {
		return err
	}
	dc := new(store.Datacenter)
	for key, value := range data {
		switch key {
		case "id":
			dc.ID = value
		case "name":
			dc.Name = value
		case "parent_id":
			fallthrough
		case "parent":
			dc.ParentID = value
		case "desc":
			fallthrough
		case "description":
			dc.Description = value
		default:
			return fmt.Errorf("Invalid token %s at line %d: %s", key, lineCount, line)
		}
	}
	li.datacenters = append(li.datacenters, dc)
	return nil
}

func (li *LocalIni) addGroup(line string) error {
	data, err := parseLine(line)
	if err != nil {
		return err
	}
	group := new(store.Group)
	for key, value := range data {
		switch key {
		case "id":
			group.ID = value
		case "name":
			group.Name = value
		case "parent_id":
			fallthrough
		case "parent":
			group.ParentID = value
		case "tags":
			group.Tags = strings.Split(value, ",")
		case "description":
			group.Description = value
		case "workgroup":
			fallthrough
		case "wg":
			fallthrough
		case "wg_id":
			group.WorkGroupID = value
		default:
			return fmt.Errorf("Invalid token %s at line %d: %s", key, lineCount, line)
		}
	}
	li.groups = append(li.groups, group)
	return nil
}

func (li *LocalIni) addHost(line string) error {
	data, err := parseLine(line)
	if err != nil {
		return err
	}
	host := new(store.Host)
	for key, value := range data {
		switch key {
		case "id":
			host.ID = value
		case "name":
			host.FQDN = value
		case "group_id":
			fallthrough
		case "group":
			host.GroupID = value
		case "aliases":
			host.Aliases = strings.Split(value, ",")
		case "tags":
			host.Tags = strings.Split(value, ",")
		case "description":
			host.Description = value
		case "datacenter":
			fallthrough
		case "datacenter_id":
			fallthrough
		case "dc":
			fallthrough
		case "dc_id":
			host.DatacenterID = value
		default:
			return fmt.Errorf("Invalid token %s at line %d: %s", key, lineCount, line)
		}
	}
	li.hosts = append(li.hosts, host)
	return nil
}
