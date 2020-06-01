package store

import (
	"sort"
	"testing"
)

type FakeBackend struct {
	hosts       []*Host
	groups      []*Group
	workgroups  []*WorkGroup
	datacenters []*Datacenter
}

func (fb *FakeBackend) Hosts() []*Host {
	return fb.hosts
}

func (fb *FakeBackend) Groups() []*Group {
	return fb.groups
}

func (fb *FakeBackend) Datacenters() []*Datacenter {
	return fb.datacenters
}

func (fb *FakeBackend) WorkGroups() []*WorkGroup {
	return fb.workgroups
}

func (fb *FakeBackend) Load() error {
	wg := &WorkGroup{
		ID:     "wg1",
		Name:   "workgroup",
		Groups: make([]*Group, 0),
	}
	fb.workgroups = append(fb.workgroups, wg)

	group1 := &Group{
		ID:          "g1",
		Name:        "group1",
		WorkGroupID: "wg1",
		ParentID:    "",
		Tags:        []string{"tag1", "tag2"},
	}

	group2 := &Group{
		ID:          "g2",
		Name:        "group2",
		WorkGroupID: "wg1",
		ParentID:    "g1",
		Tags:        []string{"tag3", "tag4"},
	}

	group3 := &Group{
		ID:          "g3",
		Name:        "group3",
		WorkGroupID: "wg1",
		ParentID:    "g1",
		Tags:        []string{"special"},
	}

	group4 := &Group{
		ID:          "g4",
		Name:        "group4",
		WorkGroupID: "wg1",
		ParentID:    "",
		Tags:        []string{},
	}

	fb.groups = append(fb.groups, group1, group2, group3, group4)

	dc1 := &Datacenter{
		ID:       "dc1",
		Name:     "datacenter1",
		ParentID: "",
	}

	dc2 := &Datacenter{
		ID:       "dc2",
		Name:     "datacenter1.1",
		ParentID: "dc1",
	}
	fb.datacenters = append(fb.datacenters, dc1, dc2)

	host := &Host{
		ID:           "h1",
		FQDN:         "host1.example.com",
		Aliases:      []string{"host1", "host1.i"},
		Tags:         []string{"tag5"},
		GroupID:      "g2",
		DatacenterID: "dc2",
	}

	host2 := &Host{
		ID:           "h2",
		FQDN:         "host2.example.com",
		Aliases:      []string{"host2", "host2.i"},
		Tags:         []string{},
		GroupID:      "g3",
		DatacenterID: "dc2",
	}

	host3 := &Host{
		ID:           "h3",
		FQDN:         "host3.example.com",
		Aliases:      []string{"host3", "host3.i"},
		Tags:         []string{},
		GroupID:      "g4",
		DatacenterID: "dc2",
	}

	host4 := &Host{
		ID:           "h4",
		FQDN:         "host4.example.com",
		Aliases:      []string{"host4", "host4.i"},
		Tags:         []string{},
		GroupID:      "g4",
		DatacenterID: "dc2",
	}

	fb.hosts = append(fb.hosts, host, host2, host3, host4)

	return nil
}

func (fb *FakeBackend) Reload() error {
	return fb.Load()
}

func newFB() *FakeBackend {
	fb := new(FakeBackend)
	fb.hosts = make([]*Host, 0)
	fb.groups = make([]*Group, 0)
	fb.datacenters = make([]*Datacenter, 0)
	fb.workgroups = make([]*WorkGroup, 0)
	return fb
}

func TestStoreRelations(t *testing.T) {
	var found bool
	fb := newFB()
	fb.Load()

	s, err := CreateStore(fb)
	if err != nil {
		t.Error(err)
		return
	}

	wg, found := s.workgroups._id["wg1"]
	if !found {
		t.Error("Workgroup wg1 not found")
	}

	g1, found := s.groups._id["g1"]
	if !found {
		t.Error("Group g1 not found")
	}

	g2, found := s.groups._id["g2"]
	if !found {
		t.Error("Group g2 not found")
	}

	h1, found := s.hosts._id["h1"]
	if !found {
		t.Error("Host h1 not found")
	}

	if g1.WorkGroup != wg {
		t.Error("Group g1 should be connected to workgroup wg1")
	}
	if g2.WorkGroup != wg {
		t.Error("Group g2 should be connected to workgroup wg1")
	}
	if g2.Parent != g1 {
		t.Error("Group g2 parent must be g1")
	}

	found = false
	for _, chg := range g1.Children {
		if chg == g2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("g2 should exist in g1.Children")
	}

	var tagsReal []string
	var tagsExpected []string

	tagsReal = g2.AllTags
	tagsExpected = []string{"tag1", "tag2", "tag3", "tag4"}
	sort.Strings(tagsReal)

	if len(tagsReal) != len(tagsExpected) {
		t.Errorf("Group2 AllTags expected to be of length %d, however its length is %d", len(tagsExpected), len(tagsReal))
		return
	}

	for i := 0; i < len(tagsExpected); i++ {
		if tagsReal[i] != tagsExpected[i] {
			t.Errorf("Expected tag %s at position %d, found %s", tagsExpected[i], i, tagsReal[i])
		}
	}

	tagsReal = h1.AllTags
	tagsExpected = []string{"tag1", "tag2", "tag3", "tag4", "tag5"}
	sort.Strings(tagsReal)
	if len(tagsReal) != len(tagsExpected) {
		t.Errorf("Host1 AllTags expected to be of length %d, however its length is %d", len(tagsExpected), len(tagsReal))
		return
	}

	for i := 0; i < len(tagsExpected); i++ {
		if tagsReal[i] != tagsExpected[i] {
			t.Errorf("Expected tag %s at position %d, found %s", tagsExpected[i], i, tagsReal[i])
		}
	}

}

func TestHostlist1(t *testing.T) {
	fb := newFB()
	fb.Load()

	s, err := CreateStore(fb)
	if err != nil {
		t.Error(err)
		return
	}

	hostlist, err := s.HostList([]rune("%group1#special"))
	if err != nil {
		t.Error(err)
		return
	}

	if len(hostlist) != 1 {
		t.Errorf("hostlist %%group1#special is expected to contain exactly 1 element, %v", hostlist)
		return
	}

	if hostlist[0] != "host2.example.com" {
		t.Errorf("host is expected to be host2.example.gom, got %s instead", hostlist[0])
	}
}

func TestHostlist2(t *testing.T) {
	fb := newFB()
	fb.Load()

	s, err := CreateStore(fb)
	if err != nil {
		t.Error(err)
		return
	}

	hostlist, err := s.HostList([]rune("%group1#tag1"))
	if err != nil {
		t.Error(err)
		return
	}

	if len(hostlist) != 2 {
		t.Errorf("hostlist %%group1#tag1 is expected to contain exactly 2 elements")
		return
	}
}

func TestExclude(t *testing.T) {
	fb := newFB()
	fb.Load()

	s, err := CreateStore(fb)
	if err != nil {
		t.Error(err)
		return
	}

	hostlist, err := s.HostList([]rune("%group4"))
	if err != nil {
		t.Error(err)
	}

	if len(hostlist) != 2 {
		t.Errorf("hostlist is expected to consist of exactly two elements, %v", hostlist)
	}

	hostlist, err = s.HostList([]rune("%group4,-host3.example.com"))
	if err != nil {
		t.Error(err)
	}

	if len(hostlist) != 1 {
		t.Errorf("hostlist is expected to consist of exactly one element, %v", hostlist)
	}

}
