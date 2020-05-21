package store

import (
	"regexp"
	"sort"
	"strings"

	"github.com/facette/natsort"
	"github.com/viert/sekwence"
	"github.com/viert/xc/stringslice"
)

// Store represents host tree data store
type Store struct {
	datacenters *dcstore
	groups      *groupstore
	hosts       *hoststore
	workgroups  *wgstore
	tags        []string
	backend     Backend

	naturalSort bool
}

func (s *Store) reinitStore() {
	s.datacenters = new(dcstore)
	s.datacenters._id = make(map[string]*Datacenter)
	s.datacenters.name = make(map[string]*Datacenter)
	s.groups = new(groupstore)
	s.groups._id = make(map[string]*Group)
	s.groups.name = make(map[string]*Group)
	s.hosts = new(hoststore)
	s.hosts._id = make(map[string]*Host)
	s.hosts.fqdn = make(map[string]*Host)
	s.workgroups = new(wgstore)
	s.workgroups._id = make(map[string]*WorkGroup)
	s.workgroups.name = make(map[string]*WorkGroup)
	s.tags = make([]string, 0)
}

func (s *Store) addHost(host *Host) {
	s.hosts.fqdn[host.FQDN] = host
	s.hosts._id[host.ID] = host
}

func (s *Store) addGroup(group *Group) {
	s.groups.name[group.Name] = group
	s.groups._id[group.ID] = group
}

func (s *Store) addDatacenter(dc *Datacenter) {
	s.datacenters.name[dc.Name] = dc
	s.datacenters._id[dc.ID] = dc
}

func (s *Store) addWorkGroup(wg *WorkGroup) {
	s.workgroups.name[wg.Name] = wg
	s.workgroups._id[wg.ID] = wg
}

// CompleteTag returns all postfixes of tags starting with a given prefix
func (s *Store) CompleteTag(prefix string) []string {
	res := make([]string, 0)
	for _, tag := range s.tags {
		if prefix == "" || strings.HasPrefix(tag, prefix) {
			res = append(res, tag[len(prefix):])
		}
	}
	sort.Strings(res)
	return res
}

// CompleteHost returns all postfixes of host fqdns starting with a given prefix
func (s *Store) CompleteHost(prefix string) []string {
	res := make([]string, 0)
	for hostname := range s.hosts.fqdn {
		if prefix == "" || strings.HasPrefix(hostname, prefix) {
			res = append(res, hostname[len(prefix):])
		}
	}
	sort.Strings(res)
	return res
}

// CompleteGroup returns all postfixes of group names starting with a given prefix
func (s *Store) CompleteGroup(prefix string) []string {
	res := make([]string, 0)
	for name := range s.groups.name {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			res = append(res, name[len(prefix):])
		}
	}
	sort.Strings(res)
	return res
}

// CompleteDatacenter returns all postfixes of dc names starting with a given prefix
func (s *Store) CompleteDatacenter(prefix string) []string {
	res := make([]string, 0)
	for name := range s.datacenters.name {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			res = append(res, name[len(prefix):])
		}
	}
	sort.Strings(res)
	return res
}

// CompleteWorkGroup returns all postfixes of workgroup names starting with a given prefix
func (s *Store) CompleteWorkGroup(prefix string) []string {
	res := make([]string, 0)
	for name := range s.workgroups.name {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			res = append(res, name[len(prefix):])
		}
	}
	sort.Strings(res)
	return res
}

func (s *Store) matchHost(pattern *regexp.Regexp) []string {
	res := make([]string, 0)
	for hostname := range s.hosts.fqdn {
		if pattern.MatchString(hostname) {
			res = append(res, hostname)
		}
	}
	sort.Strings(res)
	return res
}

func (s *Store) groupAllChildren(g *Group) []*Group {
	children := make([]*Group, len(g.Children))
	copy(children, g.Children)

	for _, child := range g.Children {
		children = append(children, s.groupAllChildren(child)...)
	}
	return children
}

func (s *Store) groupAllHosts(g *Group) []*Host {
	allGroups := s.groupAllChildren(g)
	allGroups = append(allGroups, g)
	hosts := make([]*Host, 0)
	for _, group := range allGroups {
		hosts = append(hosts, group.Hosts...)
	}
	return hosts
}

// SetNaturalSort enables/disables using of natural sorting
// within one expression token (i.e. group)
func (s *Store) SetNaturalSort(value bool) {
	s.naturalSort = value
}

// HostList returns a list of host FQDNs according to a given
// expression
func (s *Store) HostList(expr []rune) ([]string, error) {
	tokens, err := parseExpression(expr)
	if err != nil {
		return nil, err
	}

	hostlist := make([][]string, 0)

	for _, token := range tokens {
		singleTokenHosts := make([]string, 0)
		switch token.Type {
		case tTypeHostRegexp:
			for _, host := range s.matchHost(token.RegexpFilter) {
				maybeAddHost(&singleTokenHosts, host, token.Exclude)
			}
		case tTypeHost:

			hosts, err := sekwence.ExpandPattern(token.Value)
			if err != nil {
				hosts = []string{token.Value}
			}

			for _, host := range hosts {
				if len(token.TagsFilter) > 0 {
					invhost, found := s.hosts.fqdn[host]
					if !found {
						continue
					}
					for _, tag := range token.TagsFilter {
						if !stringslice.Contains(invhost.AllTags, tag) {
							continue
						}
					}
				}
				maybeAddHost(&singleTokenHosts, host, token.Exclude)
			}

		case tTypeGroup:
			if group, found := s.groups.name[token.Value]; found {
				hosts := s.groupAllHosts(group)

			hostLoop1:
				for _, host := range hosts {
					if token.DatacenterFilter != "" {
						if host.Datacenter == nil {
							continue
						}
						if host.Datacenter.Name != token.DatacenterFilter {
							// TODO tree
							continue
						}
					}

					for _, tag := range token.TagsFilter {
						if !stringslice.Contains(host.AllTags, tag) {
							continue hostLoop1
						}
					}

					if token.RegexpFilter != nil {
						if !token.RegexpFilter.Match([]byte(host.FQDN)) {
							continue
						}
					}
					maybeAddHost(&singleTokenHosts, host.FQDN, token.Exclude)
				}
			}

		case tTypeWorkGroup:
			workgroups := make([]*WorkGroup, 0)
			if token.Value == "" {
				for _, wg := range s.workgroups.name {
					workgroups = append(workgroups, wg)
				}
			} else {
				wg, found := s.workgroups.name[token.Value]
				if found {
					workgroups = []*WorkGroup{wg}
				}
			}

			if len(workgroups) > 0 {
				hosts := make([]*Host, 0)
				for _, wg := range workgroups {
					groups := wg.Groups
					for _, group := range groups {
						hosts = append(hosts, group.Hosts...)
					}
				}

			hostLoop2:
				for _, host := range hosts {
					if token.DatacenterFilter != "" {
						if host.Datacenter == nil {
							continue
						}
						if host.Datacenter.Name != token.DatacenterFilter {
							// TODO tree
							continue
						}
					}

					for _, tag := range token.TagsFilter {
						if !stringslice.Contains(host.AllTags, tag) {
							continue hostLoop2
						}
					}

					if token.RegexpFilter != nil {
						if !token.RegexpFilter.Match([]byte(host.FQDN)) {
							continue
						}
					}

					maybeAddHost(&singleTokenHosts, host.FQDN, token.Exclude)
				}
			}
		}
		if len(singleTokenHosts) > 0 {
			hostlist = append(hostlist, singleTokenHosts)
		}
	}

	results := make([]string, 0)
	for _, sthosts := range hostlist {
		// sorting within one expression token only
		// the order of tokens themselves should be respected
		if s.naturalSort {
			natsort.Sort(sthosts)
		} else {
			sort.Strings(sthosts)
		}

		for _, host := range sthosts {
			results = append(results, host)
		}
	}
	return results, nil
}

// apply is called after the raw data is loaded and creates relations
// between models according to relation ids
func (s *Store) apply() {
	var host *Host
	var group *Group
	var parent *Group
	var workgroup *WorkGroup
	var datacenter *Datacenter

	tagmap := make(map[string]bool)

	for _, dc := range s.datacenters._id {
		if dc.ParentID != "" {
			dc.Parent = s.datacenters._id[dc.ParentID]
		}
	}

	for _, dc := range s.datacenters._id {
		if dc.Parent != nil {
			datacenter = dc.Parent
			for datacenter.Parent != nil {
				datacenter = datacenter.Parent
			}
			dc.Root = datacenter
		}
	}

	for _, group = range s.groups._id {
		if group.ParentID != "" {
			parent = s.groups._id[group.ParentID]
			if parent != nil {
				group.Parent = parent
				parent.Children = append(parent.Children, group)
			}
		}

		if group.WorkGroupID != "" {
			workgroup = s.workgroups._id[group.WorkGroupID]
			if workgroup != nil {
				group.WorkGroup = workgroup
				workgroup.Groups = append(workgroup.Groups, group)
			}
		}
	}

	// calculate AllTags for groups and collect all the tags into a set
	for _, group = range s.groups._id {

		// collecting tags in one set
		for _, tag := range group.Tags {
			tagmap[tag] = true
		}

		group.AllTags = make([]string, 0)
		parent = group
		for parent != nil {
			group.AllTags = append(group.AllTags, parent.Tags...)
			parent = parent.Parent
		}
		sort.Strings(group.AllTags)
	}

	for _, host = range s.hosts._id {
		if host.GroupID != "" {
			group = s.groups._id[host.GroupID]
			if group != nil {
				host.Group = group
				group.Hosts = append(group.Hosts, host)
			}
		}
		if host.DatacenterID != "" {
			host.Datacenter = s.datacenters._id[host.DatacenterID]
		}
	}

	// calculate AllTags for hosts
	for _, host = range s.hosts._id {

		// collecting tags in one set
		for _, tag := range host.Tags {
			tagmap[tag] = true
		}

		host.AllTags = make([]string, len(host.Tags))
		copy(host.AllTags, host.Tags)
		parent = host.Group
		for parent != nil {
			host.AllTags = append(host.AllTags, parent.Tags...)
			parent = parent.Parent
		}
	}

	for tag := range tagmap {
		s.tags = append(s.tags, tag)
	}
	sort.Strings(s.tags)
}

// CreateStore creates a new store and loads data from a given backend
func CreateStore(backend Backend) (*Store, error) {
	s := new(Store)
	s.backend = backend
	s.naturalSort = true
	err := s.BackendLoad()
	return s, err
}

func (s *Store) copyBackendData() {
	s.reinitStore()
	for _, host := range s.backend.Hosts() {
		s.addHost(host)
	}
	for _, group := range s.backend.Groups() {
		s.addGroup(group)
	}
	for _, datacenter := range s.backend.Datacenters() {
		s.addDatacenter(datacenter)
	}
	for _, workgroup := range s.backend.WorkGroups() {
		s.addWorkGroup(workgroup)
	}
	s.apply()
}

// BackendLoad is a proxy to backend.Load handler
func (s *Store) BackendLoad() error {
	err := s.backend.Load()
	if err == nil {
		s.copyBackendData()
	}
	return err
}

// BackendReload is a proxy to backend.Reload handler
func (s *Store) BackendReload() error {
	err := s.backend.Reload()
	if err == nil {
		s.copyBackendData()
	}
	return err
}
