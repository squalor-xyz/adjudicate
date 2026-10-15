// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"flag"
	"strings"
)

// stringList is a repeatable string flag: every occurrence adds a value, in
// order.
type stringList []string

func (s *stringList) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// parseInterspersed parses args with fs like fs.Parse, except that flags may
// come after positional arguments (Go's flag package stops at the first
// positional). It returns the positional arguments in order. A lone "-" is
// positional, and "--" ends the flags: everything after it is positional. A
// "--" that is the value of a flag stays a value. The error is fs.Parse's, so
// -h and --help give flag.ErrHelp.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			pos = append(pos, args[i+1:]...)
			i = len(args)
		case len(a) < 2 || a[0] != '-':
			pos = append(pos, a)
		default:
			flags = append(flags, a)
			// A flag written without "=value" that takes a value owns the
			// next argument, whatever it looks like.
			if takesValue(fs, a) && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return pos, nil
}

// takesValue reports whether the flag argument a (such as "--key" or "-tag")
// needs the following argument as its value: it has no "=", and it names a
// defined flag that is not a bool flag.
func takesValue(fs *flag.FlagSet, a string) bool {
	name := strings.TrimLeft(a, "-")
	if strings.Contains(name, "=") {
		return false
	}
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !b.IsBoolFlag()
}
