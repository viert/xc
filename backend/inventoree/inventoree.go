package inventoree

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/viert/xc/config"
	"github.com/viert/xc/store"
	"github.com/viert/xc/term"
)

// New creates and cofigures inventoree-based backend
func New(cfg *config.XCConfig) (*Inventoree, error) {
	var workgroupNames []string
	options := cfg.BackendCfg.Options

	// workgroups configuration
	wgString, found := options["work_groups"]
	if !found || wgString == "" {
		workgroupNames = make([]string, 0)
	} else {
		splitExpr := regexp.MustCompile(`\s*,\s*`)
		workgroupNames = splitExpr.Split(wgString, -1)
	}

	// url configuration
	url, found := options["url"]
	if !found {
		return nil, fmt.Errorf("Inventoree backend URL is not configured")
	}

	// insecure configuration
	insec, found := options["insecure"]
	if !found {
		insec = "false"
	}

	insecure := false
	if insec == "true" || insec == "yes" || insec == "1" {
		insecure = true
		term.Warnf("WARNING: Inventory backend will be accessed in insecure mode\n")
	}

	// host key field
	hostKey, found := options["host_key_field"]
	if !found {
		hostKey = "fqdn"
	}

	if hostKey != "fqdn" && hostKey != "ssh_hostname" {
		term.Errorf("ERROR: invalid host_key_field \"%s\", only \"fqdn\" and \"ssh_hostname\" are allowed\n", hostKey)
		term.Warnf("Falling back to fqdn as host key field\n")
		hostKey = "fqdn"
	}

	// auth configuration
	authToken, found := options["auth_token"]
	if !found {
		return nil, fmt.Errorf("Inventoree auth_token option is missing")
	}

	return &Inventoree{
		workgroupNames: workgroupNames,
		cacheTTL:       cfg.CacheTTL,
		cacheDir:       cfg.CacheDir,
		url:            url,
		hostKeyField:   hostKey,
		authToken:      authToken,
		insecure:       insecure,
	}, nil
}

// Hosts exported backend method
func (i *Inventoree) Hosts() []*store.Host {
	return i.hosts
}

// Groups exported backend method
func (i *Inventoree) Groups() []*store.Group {
	return i.groups
}

// WorkGroups exported backend method
func (i *Inventoree) WorkGroups() []*store.WorkGroup {
	return i.workgroups
}

// Datacenters exported backend method
func (i *Inventoree) Datacenters() []*store.Datacenter {
	return i.datacenters
}

// Reload forces reloading data from HTTP(S)
func (i *Inventoree) Reload() error {
	err := i.loadRemote()
	if err != nil {
		term.Errorf("\n%s\n", err)
		term.Warnf("Trying to load data from cache...\n")
		// trying to use cache
		return i.loadLocal()
	}
	return nil
}

// Load tries to load data from cache unless it's expired
// In case of cache expiration or absense it triggers Reload()
func (i *Inventoree) Load() error {
	var err error
	if i.cacheExpired() {
		return i.Reload()
	}
	// trying to use cache
	err = i.loadLocal()
	if err != nil {
		// if it failed, trying to get data from remote
		return i.loadRemote()
	}
	return nil
}

func (i *Inventoree) inventoreeGet(path string) ([]byte, error) {
	client := &http.Client{}

	if i.insecure {
		rootCAs, _ := x509.SystemCertPool()
		tlsconf := &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            rootCAs,
		}
		transport := &http.Transport{TLSClientConfig: tlsconf}
		client.Transport = transport
	}

	url := fmt.Sprintf("%s%s", i.url, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-Api-Auth-Token", i.authToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Status code %d while fetching %s", resp.StatusCode, url)
	}

	return ioutil.ReadAll(resp.Body)
}

func (i *Inventoree) loadLocal() error {
	data, err := ioutil.ReadFile(i.cacheFilename())
	if err != nil {
		return err
	}
	lc := new(cache)
	err = json.Unmarshal(data, lc)
	if err != nil {
		return err
	}
	i.extractCache(lc)
	term.Warnf("Hosts loaded from cache\n")
	return nil
}

func (i *Inventoree) loadRemote() error {
	var data []byte
	var count int
	var err error

	lc := new(cache)
	lc.Datacenters = make([]*datacenter, 0)
	lc.Groups = make([]*group, 0)
	lc.WorkGroups = make([]*workgroup, 0)
	lc.Hosts = make([]*host, 0)

	term.Warnf("Loading datacenters...")
	data, err = i.inventoreeGet("/api/v2/datacenters/?_fields=_id,name,description,parent_id&_nopaging=true")
	if err != nil {
		return err
	}

	dcdata := &apiDatacenters{}
	err = json.Unmarshal(data, dcdata)
	if err != nil {
		return err
	}

	count = 0
	for _, dc := range dcdata.Data {
		lc.Datacenters = append(lc.Datacenters, dc)
		count++
	}
	term.Warnf("%d loaded\n", count)

	term.Warnf("Loading workgroups...")
	count = 0

	if len(i.workgroupNames) > 0 {
		for _, wgname := range i.workgroupNames {
			term.Warnf(wgname + "..")
			path := fmt.Sprintf("/api/v2/work_groups/%s?_fields=_id,name,description", wgname)
			data, err = i.inventoreeGet(path)
			if err != nil {
				term.Errorf("\nError loading workgroup %s: %s\n", wgname, err)
				continue
			}

			wgdata := &apiWorkgroup{}
			err = json.Unmarshal(data, wgdata)
			if err != nil {
				term.Errorf("\nError loading workgroup %s: %s\n", wgname, err)
				continue
			}
			lc.WorkGroups = append(lc.WorkGroups, wgdata.Data)
			count++
		}
	} else {
		path := fmt.Sprintf("/api/v2/work_groups/?_fields=_id,name,description&_nopaging=true")
		data, err = i.inventoreeGet(path)
		if err == nil {
			wgdata := &apiWorkgroupList{}
			err = json.Unmarshal(data, wgdata)
			if err == nil {
				for _, wg := range wgdata.Data {
					lc.WorkGroups = append(lc.WorkGroups, wg)
					count++
				}
			}
		}
		if err != nil {
			term.Errorf("\nError loading workgroups: %s\n", err)
		}
	}
	term.Warnf("%d loaded\n", count)

	term.Warnf("Loading groups...")

	count = 0
	if len(i.workgroupNames) > 0 {
		for _, wgname := range i.workgroupNames {
			path := fmt.Sprintf("/api/v2/groups/?work_group_id=%s&_fields=_id,name,parent_id,local_tags,description,work_group_id&_nopaging=true", wgname)
			data, err = i.inventoreeGet(path)
			if err != nil {
				term.Errorf("%s..", wgname)
				continue
			}
			gdata := &apiGroups{}
			err = json.Unmarshal(data, gdata)
			if err != nil {
				return err
			}
			for _, g := range gdata.Data {
				lc.Groups = append(lc.Groups, g)
				count++
			}
			term.Warnf("%s..", wgname)
		}
	} else {
		path := "/api/v2/groups/?_fields=_id,name,parent_id,local_tags,description,work_group_id&_nopaging=true"
		data, err = i.inventoreeGet(path)
		if err != nil {
			return err
		}

		gdata := &apiGroups{}
		err = json.Unmarshal(data, gdata)
		if err != nil {
			return err
		}
		for _, g := range gdata.Data {
			lc.Groups = append(lc.Groups, g)
			count++
		}
	}
	term.Warnf("%d loaded\n", count)

	term.Warnf("Loading hosts...")
	count = 0

	fieldSet := "_id,fqdn,ssh_hostname,local_tags,group_id,datacenter_id,aliases,description"
	moveToFQDN := i.hostKeyField != "fqdn"

	for _, wg := range lc.WorkGroups {
		path := fmt.Sprintf("/api/v2/hosts/?work_group_id=%s&_fields=%s&_nopaging=true", wg.ID, fieldSet)
		data, err = i.inventoreeGet(path)
		if err != nil {
			term.Errorf("\nError loading hosts of work group %s: %s", wg.Name, err)
			continue
		}
		hdata := &apiHosts{}
		err = json.Unmarshal(data, hdata)
		if err != nil {
			term.Errorf("\nError loading hosts of work group %s: %s", wg.Name, err)
			continue
		}
		for _, h := range hdata.Data {
			if moveToFQDN && h.SSHHostname != "" {
				// copying ssh_hostname to FQDN
				// to keep things simple
				h.FQDN = h.SSHHostname
			}
			lc.Hosts = append(lc.Hosts, h)
			count++
		}
		term.Warnf(wg.Name + "..")
	}
	term.Warnf("%d loaded\n", count)
	err = i.saveCache(lc)
	if err != nil {
		term.Errorf("Error saving cacne: %s\n", err)
	} else {
		term.Successf("Cache saved to %s\n", i.cacheFilename())
	}
	i.extractCache(lc)
	return nil
}

func (i *Inventoree) cacheExpired() bool {
	st, err := os.Stat(i.cacheFilename())
	if err != nil {
		if os.IsNotExist(err) {
			// no cache in general means that it's been expired
			return true
		}
	}
	modifiedAt := st.ModTime()
	return modifiedAt.Add(i.cacheTTL).Before(time.Now())
}

func (i *Inventoree) cacheFilename() string {
	var wglist string
	if len(i.workgroupNames) > 0 {
		wglist = strings.Join(i.workgroupNames, "_")
	} else {
		wglist = "all"
	}
	fn := fmt.Sprintf("inv_cache_%s.json", wglist)
	return path.Join(i.cacheDir, fn)
}

func (i *Inventoree) saveCache(lc *cache) error {
	_, err := os.Stat(i.cacheDir)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(i.cacheDir, 0755)
		if err != nil {
			return fmt.Errorf("Error creating cache dir: %s", err)
		}
	}
	f, err := os.Create(i.cacheFilename())
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(lc)
	if err != nil {
		return err
	}
	f.Write(data)
	return nil
}

func (i *Inventoree) extractCache(lc *cache) {
	i.datacenters = make([]*store.Datacenter, 0)
	i.workgroups = make([]*store.WorkGroup, 0)
	i.groups = make([]*store.Group, 0)
	i.hosts = make([]*store.Host, 0)

	for _, dc := range lc.Datacenters {
		i.datacenters = append(i.datacenters, &store.Datacenter{
			ID:          dc.ID,
			Name:        dc.Name,
			Description: dc.Description,
			ParentID:    dc.ParentID,
		})
	}

	for _, wg := range lc.WorkGroups {
		i.workgroups = append(i.workgroups, &store.WorkGroup{
			ID:          wg.ID,
			Name:        wg.Name,
			Description: wg.Description,
		})
	}

	for _, g := range lc.Groups {
		i.groups = append(i.groups, &store.Group{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			ParentID:    g.ParentID,
			Tags:        g.Tags,
			WorkGroupID: g.WorkGroupID,
		})
	}

	for _, h := range lc.Hosts {
		i.hosts = append(i.hosts, &store.Host{
			ID:           h.ID,
			FQDN:         h.FQDN,
			Aliases:      h.Aliases,
			Tags:         h.Tags,
			GroupID:      h.GroupID,
			DatacenterID: h.DatacenterID,
		})
	}
}
