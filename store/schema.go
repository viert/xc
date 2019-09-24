package store

// Datacenter represents datacenter object
type Datacenter struct {
	ID          string `json:"name"`
	Description string
	Name        string
	ParentID    string `json:"parent"`

	Parent   *Datacenter
	Root     *Datacenter
	Children []*Datacenter
}

// Host represents host object
type Host struct {
	ID           string `json:"name"`
	Aliases      []string
	Tags         []string
	FQDN         string
	GroupID      string `json:"group"`
	DatacenterID string `json:"dc"`
	Description  string

	AllTags    []string
	Datacenter *Datacenter
	Group      *Group
}

// Group represents a group of hosts
type Group struct {
	ID          string `json:"name"`
	ParentID    string `json:"parent"`
	Tags        []string
	Description string
	Name        string
	WorkGroupID string `json:"workgroup"`

	AllTags   []string
	WorkGroup *WorkGroup
	Children  []*Group
	Parent    *Group
	Hosts     []*Host
}

// WorkGroup represents a group of users
type WorkGroup struct {
	ID          string `json:"name"`
	Name        string
	Description string
	Groups      []*Group
}

type dcstore struct {
	_id  map[string]*Datacenter
	name map[string]*Datacenter
}

type groupstore struct {
	_id  map[string]*Group
	name map[string]*Group
}

type hoststore struct {
	_id  map[string]*Host
	fqdn map[string]*Host
}

type wgstore struct {
	_id  map[string]*WorkGroup
	name map[string]*WorkGroup
}
