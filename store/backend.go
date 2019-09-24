package store

// Backend represents a store backend interface
type Backend interface {
	Load() error
	Reload() error

	Datacenters() []*Datacenter
	Groups() []*Group
	WorkGroups() []*WorkGroup
	Hosts() []*Host
}
