package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
)

// A baseline is the size of each function over a limit, as it was: one
// may stay as large, or shrink, but not grow. A line of its file is the
// size, the directory, the name, and the metric but for the ABC size:
//
//	17.7 kvs Table.recover
//	4 kvs Table.recover nesting
type baseline map[string]float64

// readBaseline reads the baseline of path; none if the file is missing.
func readBaseline(path string) (baseline, error) {
	b := baseline{}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	for i, line := range strings.Split(string(data), "\n") {
		if err := b.parse(line); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
		}
	}
	return b, nil
}

// parse adds the entry of line, if any: blank lines and # comments have
// none.
func (b baseline) parse(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	value, key, ok := strings.Cut(line, " ")
	if !ok {
		return fmt.Errorf("not a size and a function: %q", line)
	}
	x, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	b[key] = max(b[key], x) // init may be there twice
	return nil
}

// write writes the baseline of the sizes over their limit, or left out,
// to path.
func write(path string, zs []size) error {
	b := baseline{}
	for _, z := range zs {
		if z.kept() {
			b[z.key] = max(b[z.key], z.value)
		}
	}
	keys := make([]string, 0, len(b))
	for k := range b {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var w strings.Builder
	w.WriteString("# The size of each function over a limit, or left out by its directive,\n")
	w.WriteString("# which it may not pass:\n")
	w.WriteString("# see github.com/shun159/abc. Rewritten with -write.\n")
	for _, k := range keys {
		fmt.Fprintf(&w, "%g %s\n", b[k], k)
	}
	return os.WriteFile(path, []byte(w.String()), 0o644)
}

// A problem is a function grown past a limit, or past its baseline, or
// left out of a limit and not in the baseline.
type problem struct {
	z     size
	bound float64 // the limit, or its baseline
	what  string
}

// What a problem is past.
const (
	pastLimit    = "the limit"
	pastBaseline = "its baseline"
	unrecorded   = "not in the baseline"
)

// check returns the sizes over their limit that are not in b, or larger
// than their entry, and those left out not in b, the furthest over their
// limit first.
func check(zs []size, b baseline) []problem {
	var ps []problem
	for _, z := range zs {
		if !z.kept() {
			continue
		}
		was, known := b[z.key]
		switch {
		case !known && z.leftOut:
			ps = append(ps, problem{z, 0, unrecorded})
		case !known:
			ps = append(ps, problem{z, z.m.limit, pastLimit})
		case z.value > was:
			ps = append(ps, problem{z, was, pastBaseline})
		}
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].z.ratio() > ps[j].z.ratio() })
	return ps
}

// shrunk returns the entries of b smaller now, or gone, to write anew.
func shrunk(zs []size, b baseline) []string {
	now := map[string]float64{}
	for _, z := range zs {
		now[z.key] = max(now[z.key], z.value)
	}
	var keys []string
	for k, was := range b {
		if now[k] < was {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}
