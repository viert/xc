package cli

import (
	"fmt"
	"strconv"

	"github.com/viert/xc/term"
)

type alias struct {
	name  string
	proxy string
}

func (c *Cli) createAlias(name []rune, proxy []rune) error {
	al := &alias{string(name), string(proxy)}
	if _, found := c.aliases[al.name]; !found {
		for _, cmd := range c.completer.cmds {
			if cmd == al.name {
				return fmt.Errorf("Can't create alias \"%s\": such command already exists", al.name)
			}
		}
	}

	c.aliases[al.name] = al
	c.handlers[al.name] = c.runAlias
	c.completer.cmds = append(c.completer.cmds, al.name)
	return nil
}

func (c *Cli) runAlias(name string, argsLine string, args ...string) {
	c.aliasRecursionCount--
	if c.aliasRecursionCount < 0 {
		term.Errorf("Maximum recursion reached for alias referencing\n")
		return
	}

	al, found := c.aliases[name]
	if !found {
		term.Errorf("Alias \"%s\" is defined but not found, this must be a bug\n", name)
		return
	}

	cmdLine, err := exterpolate(al, argsLine, args...)
	if err != nil {
		term.Errorf("Error running alias \"%s\": %s\n", al.name, err)
		return
	}
	c.OneCmd(cmdLine)
}

func exterpolate(al *alias, argsLine string, args ...string) (string, error) {
	res := ""
	for i := 0; i < len(al.proxy); i++ {
		if i < len(al.proxy)-1 && al.proxy[i] == '#' {
			an, err := strconv.ParseInt(string(al.proxy[i+1]), 10, 64)
			if err == nil {
				argNum := int(an - 1)
				if argNum >= len(args) {
					return "", fmt.Errorf("alias \"%s\" needs argument #%d but only %d arguments are given", al.name, int(an), len(args))
				}
				res += args[argNum]
				i++
				continue
			}
		} else if al.proxy[i+1] == '*' {
			res += argsLine
			i++
			continue
		}
		res += string(al.proxy[i])
	}
	return res, nil
}

func (c *Cli) removeAlias(name []rune) error {
	sname := string(name)
	_, found := c.aliases[sname]
	if !found {
		return fmt.Errorf("alias \"%s\" not found", sname)
	}

	delete(c.aliases, sname)
	delete(c.handlers, sname)
	c.completer.removeCommand(sname)
	return nil
}
