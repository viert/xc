package inventoree

import (
	"time"

	"github.com/viert/xc/store"
)

// Inventoree is inventoree backend based on v2 API
type Inventoree struct {
	workgroupNames []string
	cacheTTL       time.Duration
	cacheDir       string
	url            string
	authToken      string
	insecure       bool
	hosts          []*store.Host
	groups         []*store.Group
	workgroups     []*store.WorkGroup
	datacenters    []*store.Datacenter
}

type datacenter struct {
	ID          string `json:"_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    string `json:"parent_id"`
}

type workgroup struct {
	ID          string `json:"_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type host struct {
	ID           string   `json:"_id"`
	FQDN         string   `json:"fqdn"`
	Description  string   `json:"description"`
	Tags         []string `json:"local_tags"`
	Aliases      []string `json:"aliases"`
	GroupID      string   `json:"group_id"`
	DatacenterID string   `json:"datacenter_id"`
}

type group struct {
	ID          string   `json:"_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"local_tags"`
	WorkGroupID string   `json:"work_group_id"`
	ParentID    string   `json:"parent_id"`
}

type cache struct {
	Datacenters []*datacenter `json:"datacenters"`
	Groups      []*group      `json:"groups"`
	WorkGroups  []*workgroup  `json:"work_groups"`
	Hosts       []*host       `json:"hosts"`
}

type apiDatacenters struct {
	Data []*datacenter `json:"data"`
}

type apiWorkgroup struct {
	Data *workgroup `json:"data"`
}

type apiWorkgroupList struct {
	Data []*workgroup `json:"data"`
}

type apiHosts struct {
	Data []*host `json:"data"`
}

type apiGroups struct {
	Data []*group `json:"data"`
}
