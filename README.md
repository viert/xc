# xc

XC is a parallel executer written in Go. It offers you a nice CLI with a number of handy commands to run tasks on servers in parallel, autocompletion, a simple DSL to express host lists in a laconic way and things like that.

Try start with running xc and entering the `help` command. The most useful commands are `exec`, `runscript`, `hostlist` and `distribute`. Feel free to use `help <command or topic>` to get more info on those.

## Building

Xc is structure using go modules so the building process is as easy as typing `go build -o xc cmd/xc/main.go`

## DSL

In xc hosts are combined into groups and may belong to a datacenter, groups may be combined into other groups, and any group may belong to a workgroup (you may think of workgroups as of projects), which reflects _inventoree_ storage structure.

A reference note: _Inventoree_, previously known as _conductor_ was originally developed as a http backend for one of early versions of xc (called _executer_ those days).

Every host list may be written as a expression of comma-separated tokens, where every token represents a host, a group of host, or a whole workgroup of groups of hosts.

`host1.example.com` is a single host.

`%group1` represents a group.

`*workgroup` represents a workgroup.

`@some_dc` may be postfixed to a token to filter the resulting hostlist by a datacenter

`#tag1` may be postfixed to a token to filter the result by a given tag

Any token may be excluded if it starts with `-` symbol.

```
    Some self-explanatory examples:
        host1,host2                         - simple host list containing 2 hosts
        %group1                             - a group of hosts taken from inventoree
        %group1,host1                       - all hosts from group1, plus host1
        %group1,-host2                      - all hosts from group1, excluding(!) host2
        %group2@dc1                         - all hosts from group2, located in datacenter dc1
        *myworkgroup@dc2,-%group3,host5     - all hosts from wg "myworkgroup" excluding hosts from group3, plus host5
        %group5#tag1                        - all hosts from group5 tagged with tag1
```

## Backends

At the moment xc supports 3 backends to load hosts/groups data from

### Ini file

The most easy to start with is "ini" backend. Configuration is as simple as these three lines:

```
[backend]
type = ini
filename = ~/xcdata.ini
```

The format of the ini-file itself is simple but flexible as you can see in the following self-explanatory example

```
[datacenters]
dc1
dc2

[workgroups]
workgroup1

[groups]
group1 wg=workgroup1 tags=tag1,tag2
group1.1 wg=workgroup1 parent=group1 tags=tag3,tag4

[hosts]
host1.example.com group=group1 dc=dc1
host2.example.com group=group1.1 dc=dc2
```

All the fields given in equation format, i.e., groups/dcs/tags for hosts or worgroups/tags for groups are optional.

### Conductor (Legacy Inventoree)

**Conductor** backend uses legacy v1 API of Conductor/Inventoree 5.x-6.x. This API doesn't require authentication
and provides the whole data xc may use via single handler `/api/v1/open/executer_data`. It's easy to configure as follows:

```
[backend]
type = conductor
# url is the base url of the conductor/inventoree instance
url = http://c.inventoree.ru
# you can optionally restrict the data to specified work groups
work_groups = workgroup1,workgroup2
```

### Inventoree

**Inventoree** backend utilizes the modern inventoree API v2 which is supported since inventoree 7.0. There's no specific handler for xc though so the data loading is performed in several steps. It's still faster than conductor backend as it doesn't rely on internal inventoree recursive data fetching. The configuration is similar to **conductor** however it's mandatory to configure `auth_token` option is inventoree doesn't have API handlers without authentication.

```
[backend]
type = inventoree
url = http://v7.inventoree.ru
work_groups = ...
auth_token = ...
```
