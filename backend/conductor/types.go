package conductor

import (
	"time"

	"github.com/viert/xc/store"
)

// Conductor is a backend based on Inventoree legacy v1 API
type Conductor struct {
	workgroupNames []string
	cacheTTL       time.Duration
	cacheDir       string
	url            string
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
	Tags         []string `json:"tags"`
	Aliases      []string `json:"aliases"`
	GroupID      string   `json:"group_id"`
	DatacenterID string   `json:"datacenter_id"`
}

type group struct {
	ID          string   `json:"_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	WorkGroupID string   `json:"work_group_id"`
	ParentIDs   []string `json:"parent_ids"`
}

type cache struct {
	Datacenters []*datacenter `json:"datacenters"`
	Groups      []*group      `json:"groups"`
	WorkGroups  []*workgroup  `json:"work_groups"`
	Hosts       []*host       `json:"hosts"`
}

type api struct {
	Data *cache `json:"data"`
}
