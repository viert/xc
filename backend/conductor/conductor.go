package conductor

import (
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

// New creates a new instance of Conductor backend
func New(cfg *config.XCConfig) (*Conductor, error) {
	c := &Conductor{
		cacheTTL:    cfg.CacheTTL,
		cacheDir:    cfg.CacheDir,
		hosts:       make([]*store.Host, 0),
		groups:      make([]*store.Group, 0),
		workgroups:  make([]*store.WorkGroup, 0),
		datacenters: make([]*store.Datacenter, 0),
	}

	options := cfg.BackendCfg.Options
	// workgroups configuration
	wgString, found := options["work_groups"]
	if !found || wgString == "" {
		c.workgroupNames = make([]string, 0)
	} else {
		splitExpr := regexp.MustCompile(`\s*,\s*`)
		c.workgroupNames = splitExpr.Split(wgString, -1)
	}

	// url configuration
	url, found := options["url"]
	if !found {
		return nil, fmt.Errorf("Inventoree backend URL is not configured")
	}

	c.url = url
	return c, nil

}

// Hosts exported backend method
func (c *Conductor) Hosts() []*store.Host {
	return c.hosts
}

// Groups exported backend method
func (c *Conductor) Groups() []*store.Group {
	return c.groups
}

// WorkGroups exported backend method
func (c *Conductor) WorkGroups() []*store.WorkGroup {
	return c.workgroups
}

// Datacenters exported backend method
func (c *Conductor) Datacenters() []*store.Datacenter {
	return c.datacenters
}

// Reload forces reloading data from HTTP(S)
func (c *Conductor) Reload() error {
	err := c.loadRemote()
	if err != nil {
		// trying to use cache
		return c.loadLocal()
	}
	return nil
}

// Load tries to load data from cache unless it's expired
// In case of cache expiration or absense it triggers Reload()
func (c *Conductor) Load() error {
	var err error
	if c.cacheExpired() {
		return c.Reload()
	}
	// trying to use cache
	err = c.loadLocal()
	if err != nil {
		// if it failed, trying to get data from remote
		return c.loadRemote()
	}
	return nil
}

func (c *Conductor) loadLocal() error {
	data, err := ioutil.ReadFile(c.cacheFilename())
	if err != nil {
		return err
	}
	lc := new(cache)
	err = json.Unmarshal(data, lc)
	if err != nil {
		return err
	}
	c.extractCache(lc)
	term.Warnf("Hosts loaded from cache\n")
	return nil
}

func (c *Conductor) cacheExpired() bool {
	st, err := os.Stat(c.cacheFilename())
	if err != nil {
		if os.IsNotExist(err) {
			// no cache in general means that it's been expired
			return true
		}
	}
	modifiedAt := st.ModTime()
	return modifiedAt.Add(c.cacheTTL).Before(time.Now())
}

func (c *Conductor) cacheFilename() string {
	var wglist string
	if len(c.workgroupNames) > 0 {
		wglist = strings.Join(c.workgroupNames, "_")
	} else {
		wglist = "all"
	}
	fn := fmt.Sprintf("cnd_cache_%s.json", wglist)
	return path.Join(c.cacheDir, fn)
}

func (c *Conductor) saveCache(lc *cache) error {
	_, err := os.Stat(c.cacheDir)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(c.cacheDir, 0755)
		if err != nil {
			return fmt.Errorf("Error creating cache dir: %s", err)
		}
	}
	f, err := os.Create(c.cacheFilename())
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

func (c *Conductor) extractCache(lc *cache) {
	c.datacenters = make([]*store.Datacenter, 0)
	c.workgroups = make([]*store.WorkGroup, 0)
	c.groups = make([]*store.Group, 0)
	c.hosts = make([]*store.Host, 0)

	for _, dc := range lc.Datacenters {
		c.datacenters = append(c.datacenters, &store.Datacenter{
			ID:          dc.ID,
			Name:        dc.Name,
			Description: dc.Description,
			ParentID:    dc.ParentID,
		})
	}

	for _, wg := range lc.WorkGroups {
		c.workgroups = append(c.workgroups, &store.WorkGroup{
			ID:          wg.ID,
			Name:        wg.Name,
			Description: wg.Description,
		})
	}

	for _, g := range lc.Groups {
		group := &store.Group{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Tags:        g.Tags,
			WorkGroupID: g.WorkGroupID,
		}
		if len(g.ParentIDs) > 0 {
			group.ParentID = g.ParentIDs[0]
		}
		c.groups = append(c.groups, group)
	}

	for _, h := range lc.Hosts {
		c.hosts = append(c.hosts, &store.Host{
			ID:           h.ID,
			FQDN:         h.FQDN,
			Aliases:      h.Aliases,
			Tags:         h.Tags,
			GroupID:      h.GroupID,
			DatacenterID: h.DatacenterID,
		})
	}
}

func (c *Conductor) httpGet(path string) ([]byte, error) {
	client := &http.Client{}

	url := fmt.Sprintf("%s%s", c.url, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Status code %d while fetching %s", resp.StatusCode, url)
	}

	return ioutil.ReadAll(resp.Body)
}

func (c *Conductor) loadRemote() error {
	var data []byte
	var err error

	term.Warnf("Loading executer data...\n")
	path := "/api/v1/open/executer_data"
	if len(c.workgroupNames) > 0 {
		wglist := strings.Join(c.workgroupNames, ",")
		path += fmt.Sprintf("?work_groups=%s", wglist)
	}
	data, err = c.httpGet(path)
	if err != nil {
		return err
	}

	apiResponse := new(api)
	err = json.Unmarshal(data, apiResponse)
	if err != nil {
		return err
	}

	lc := apiResponse.Data
	err = c.saveCache(lc)
	if err != nil {
		term.Errorf("Error saving cacne: %s\n", err)
	} else {
		term.Successf("Cache saved to %s\n", c.cacheFilename())
	}
	c.extractCache(lc)
	return nil
}
