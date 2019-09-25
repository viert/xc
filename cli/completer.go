package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/viert/xc/store"
	"github.com/viert/xc/stringslice"
)

type completeFunc func([]rune) ([][]rune, int)

type completer struct {
	cmds     []string
	handlers map[string]completeFunc
	store    *store.Store
}

func newCompleter(store *store.Store, commands []string) *completer {
	x := &completer{commands, make(map[string]completeFunc), store}
	x.handlers["mode"] = staticCompleter([]string{"collapse", "serial", "parallel"})
	x.handlers["debug"] = onOffCompleter()
	x.handlers["progressbar"] = onOffCompleter()
	x.handlers["prepend_hostnames"] = onOffCompleter()
	x.handlers["use_password_manager"] = onOffCompleter()
	x.handlers["raise"] = staticCompleter([]string{"none", "su", "sudo"})
	x.handlers["interpreter"] = staticCompleter([]string{"none", "su", "sudo"})
	x.handlers["exec"] = x.completeExec
	x.handlers["s_exec"] = x.completeExec
	x.handlers["c_exec"] = x.completeExec
	x.handlers["p_exec"] = x.completeExec
	x.handlers["ssh"] = x.completeExec
	x.handlers["hostlist"] = x.completeExec
	x.handlers["cd"] = completeFiles
	x.handlers["output"] = completeFiles
	x.handlers["distribute"] = x.completeDistribute
	x.handlers["runscript"] = x.completeDistribute
	x.handlers["s_runscript"] = x.completeDistribute
	x.handlers["c_runscript"] = x.completeDistribute
	x.handlers["p_runscript"] = x.completeDistribute
	x.handlers["distribute_type"] = staticCompleter([]string{"tar", "scp"})

	helpTopics := append(commands, "expressions", "config", "rcfiles", "passmgr")
	x.handlers["help"] = staticCompleter(helpTopics)
	return x
}

func split(line []rune) ([]rune, []rune) {
	strline := string(line)
	tokens := whitespace.Split(strline, 2)
	if len(tokens) < 2 {
		return []rune(tokens[0]), nil
	}
	return []rune(tokens[0]), []rune(tokens[1])
}

func runes(src []string) (dst [][]rune) {
	dst = make([][]rune, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = []rune(src[i])
	}
	return
}

func runeIndex(line []rune, sym rune) int {
	for i := 0; i < len(line); i++ {
		if line[i] == sym {
			return i
		}
	}
	return -1
}

func staticCompleter(options []string) completeFunc {
	sort.Strings(options)
	return func(line []rune) ([][]rune, int) {
		ll := len(line)
		sr := make([]string, 0)
		for _, option := range options {
			if strings.HasPrefix(option, string(line)) {
				sr = append(sr, option[ll:])
			}
		}
		return runes(sr), ll
	}
}

func onOffCompleter() completeFunc {
	return staticCompleter([]string{"on", "off"})
}

func completeFiles(line []rune) ([][]rune, int) {
	ll := len(line)
	path := string(line)
	files, err := filepath.Glob(path + "*")
	if err != nil {
		return [][]rune{}, len(line)
	}

	results := make([][]rune, len(files))
	for i := 0; i < len(files); i++ {
		filename := files[i]
		if st, err := os.Stat(filename); err == nil {
			if st.IsDir() {
				filename += "/"
			}
		}
		results[i] = []rune(filename[ll:])
	}

	return results, ll
}

func (x *completer) complete(line []rune) ([][]rune, int) {
	cmd, args := split(line)
	if args == nil {
		return x.completeCommand(cmd)
	}

	if handler, found := x.handlers[string(cmd)]; found {
		return handler(args)
	}

	return [][]rune{}, 0
}

func (x *completer) completeCommand(line []rune) ([][]rune, int) {
	sr := make([]string, 0)
	for _, cmd := range x.cmds {
		if strings.HasPrefix(cmd, string(line)) {
			sr = append(sr, cmd[len(line):]+" ")
		}
	}
	sort.Strings(sr)
	return runes(sr), len(line)
}

func (x *completer) completeDistribute(line []rune) ([][]rune, int) {
	_, cmd := split(line)
	if cmd == nil {
		return x.completeExec(line)
	}
	return completeFiles(cmd)
}

func (x *completer) completeExec(line []rune) ([][]rune, int) {
	_, shellCmd := split(line)
	if shellCmd != nil {
		return [][]rune{}, 0
	}

	// are we in complex pattern? look for comma
	ci := runeIndex(line, ',')
	if ci >= 0 {
		return x.completeExec(line[ci+1:])
	}

	// we are exactly in the beginning of the last expression
	if len(line) > 0 && line[0] == '-' {
		// exclusion is excluded from completion
		return x.completeExec(line[1:])
	}

	if len(line) > 0 && line[0] == '%' {
		return x.completeGroup(line[1:])
	}

	if len(line) > 0 && line[0] == '*' {
		return x.completeWorkGroup(line[1:])
	}

	if len(line) > 0 && line[0] == '#' {
		return x.completeTag(line[1:])
	}

	return x.completeHost(line)
}

func (x *completer) completeWorkGroup(line []rune) ([][]rune, int) {
	ai := runeIndex(line, '@')
	if ai >= 0 {
		return x.completeDatacenter(line[ai+1:])
	}
	ti := runeIndex(line, '#')
	if ti >= 0 {
		return x.completeTag(line[ti+1:])
	}
	workgroups := x.store.CompleteWorkGroup(string(line))
	return runes(workgroups), len(line)
}

func (x *completer) completeGroup(line []rune) ([][]rune, int) {
	ai := runeIndex(line, '@')
	if ai >= 0 {
		return x.completeDatacenter(line[ai+1:])
	}
	ti := runeIndex(line, '#')
	if ti >= 0 {
		return x.completeTag(line[ti+1:])
	}
	groups := x.store.CompleteGroup(string(line))
	return runes(groups), len(line)
}

func (x *completer) completeDatacenter(line []rune) ([][]rune, int) {
	datacenters := x.store.CompleteDatacenter(string(line))
	return runes(datacenters), len(line)
}

func (x *completer) completeHost(line []rune) ([][]rune, int) {
	hosts := x.store.CompleteHost(string(line))
	return runes(hosts), len(line)
}

func (x *completer) completeTag(line []rune) ([][]rune, int) {
	tags := x.store.CompleteTag(string(line))
	return runes(tags), len(line)
}

func (x *completer) Do(line []rune, pos int) ([][]rune, int) {
	postfix := line[pos:]
	result, length := x.complete(line[:pos])
	if len(postfix) > 0 {
		for i := 0; i < len(result); i++ {
			result[i] = append(result[i], postfix...)
		}
	}
	return result, length
}

func (x *completer) removeCommand(name string) {
	idx := stringslice.Index(x.cmds, name)
	if idx < 0 {
		return
	}
	x.cmds = append(x.cmds[:idx], x.cmds[idx+1:]...)
}
