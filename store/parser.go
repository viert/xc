package store

import (
	"fmt"
	"regexp"
	"strings"
)

type tokenType int
type parserstate int

const (
	tTypeHost tokenType = iota
	tTypeGroup
	tTypeWorkGroup
	tTypeHostRegexp
)

const (
	stateWait parserstate = iota
	stateReadHost
	stateReadGroup
	stateReadWorkGroup
	stateReadDatacenter
	stateReadTag
	stateReadHostBracePattern
	stateReadRegexp
)

type token struct {
	Type             tokenType
	Value            string
	DatacenterFilter string
	TagsFilter       []string
	RegexpFilter     *regexp.Regexp
	Exclude          bool
}

var (
	hostSymbols = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-{}"
)

func newToken() *token {
	ct := new(token)
	ct.TagsFilter = make([]string, 0)
	ct.RegexpFilter = nil
	return ct
}

func parseExpression(expr []rune) ([]*token, error) {
	ct := newToken()
	res := make([]*token, 0)
	state := stateWait
	tag := ""
	re := ""
	last := false
	for i := 0; i < len(expr); i++ {
		sym := expr[i]
		last = i == len(expr)-1
		switch state {
		case stateWait:
			if sym == '-' {
				ct.Exclude = true
				continue
			}

			if sym == '*' {
				state = stateReadWorkGroup
				ct.Type = tTypeWorkGroup
				continue
			}

			if sym == '%' {
				state = stateReadGroup
				ct.Type = tTypeGroup
				continue
			}

			if sym == '#' {
				ct.Type = tTypeWorkGroup
				state = stateReadTag
				tag = ""
				continue
			}

			if sym == '/' || sym == '~' {
				state = stateReadHost
				ct.Type = tTypeHostRegexp
				continue
			}

			if strings.ContainsRune(hostSymbols, sym) {
				state = stateReadHost
				ct.Type = tTypeHost
				ct.Value += string(sym)
				continue
			}

			return nil, fmt.Errorf("Invalid symbol %s, expected -, *, %% or a hostname at position %d", string(sym), i)

		case stateReadGroup:

			if sym == '@' {
				state = stateReadDatacenter
				continue
			}

			if sym == '#' {
				state = stateReadTag
				tag = ""
				continue
			}

			if sym == '/' {
				state = stateReadRegexp
				re = ""
				continue
			}

			if sym == ',' || last {
				if last && sym != ',' {
					ct.Value += string(sym)
				}

				if ct.Value == "" {
					return nil, fmt.Errorf("Empty group name at position %d", i)
				}
				res = append(res, ct)
				ct = newToken()
				state = stateWait
				continue
			}

			ct.Value += string(sym)

		case stateReadWorkGroup:

			if sym == '@' {
				state = stateReadDatacenter
				continue
			}

			if sym == '#' {
				tag = ""
				state = stateReadTag
				continue
			}

			if sym == '/' {
				state = stateReadRegexp
				re = ""
				continue
			}

			if sym == ',' || last {
				if last && sym != ',' {
					ct.Value += string(sym)
				}
				res = append(res, ct)
				ct = newToken()
				state = stateWait
				continue
			}

			ct.Value += string(sym)

		case stateReadRegexp:
			if sym == '\\' && !last && expr[i+1] == '/' {
				// screened slash
				re += "/"
				i++
				continue
			}

			if sym == '/' {
				compiled, err := regexp.Compile(re)
				if err != nil {
					return nil, fmt.Errorf("error compiling regexp at %d: %s", i, err)
				}
				ct.RegexpFilter = compiled

				res = append(res, ct)
				ct = newToken()
				state = stateWait
				// regexp should stop with '/EOL' or with '/,'
				// however stateWait doesn't expect a comma, so
				// we skip it:
				if !last && expr[i+1] == ',' {
					i++
				}
				continue
			}
			re += string(sym)

		case stateReadHost:
			if sym == '/' {
				state = stateReadRegexp
				re = ""
				continue
			}

			if sym == '{' {
				state = stateReadHostBracePattern
			}

			if sym == ',' || last {
				if last && sym != ',' {
					ct.Value += string(sym)
				}
				res = append(res, ct)
				ct = newToken()
				state = stateWait
				continue
			}

			ct.Value += string(sym)
		case stateReadHostBracePattern:
			if sym == '{' {
				return nil, fmt.Errorf("nested patterns are not allowed (at %d)", i)
			}
			if sym == '}' {
				state = stateReadHost
			}
			ct.Value += string(sym)

		case stateReadDatacenter:
			if sym == ',' || last {
				if last && sym != ',' {
					ct.DatacenterFilter += string(sym)
				}
				res = append(res, ct)
				ct = newToken()
				state = stateWait
				continue
			}

			if sym == '#' {
				tag = ""
				state = stateReadTag
				continue
			}

			if sym == '/' {
				re = ""
				state = stateReadRegexp
				continue
			}

			ct.DatacenterFilter += string(sym)

		case stateReadTag:

			if sym == ',' || last {
				if last && sym != ',' {
					tag += string(sym)
				}
				if tag == "" {
					return nil, fmt.Errorf("empty tag at position %d", i)
				}

				ct.TagsFilter = append(ct.TagsFilter, tag)
				res = append(res, ct)
				ct = newToken()
				state = stateWait
				continue
			}

			if sym == '#' {
				if tag == "" {
					return nil, fmt.Errorf("Empty tag at position %d", i)
				}
				ct.TagsFilter = append(ct.TagsFilter, tag)
				tag = ""
				continue
			}

			tag += string(sym)
		}

	}

	if ct.Value != "" || state == stateReadWorkGroup {
		// workgroup token can be empty
		res = append(res, ct)
	} else {
		if state != stateWait {
			return nil, fmt.Errorf("unexpected end of expression")
		}
	}

	if state == stateReadDatacenter || state == stateReadTag || state == stateReadHostBracePattern || state == stateReadRegexp {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	return res, nil
}
