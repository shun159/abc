package main

import (
	"fmt"
	"go/ast"
	"slices"
	"strings"
)

// disableDirective is the comment on a function that leaves metrics of it
// out, with the reason after them: a hot path measured slower once split,
// a dispatch of many short cases.
//
//	//abc:disable abc,lines the hot path of every write, under mu
const disableDirective = "//abc:disable"

// goneDirective left a whole function out: a dispatch now counts its cases
// as one decision, and what is left large is named with disable.
const goneDirective = "//abc:dispatch"

// disabled returns the metrics the directive on fn leaves out, of known,
// if it has one.
func disabled(fn *ast.FuncDecl, known []string) ([]string, error) {
	if fn.Doc == nil {
		return nil, nil
	}
	for _, c := range fn.Doc.List {
		if strings.HasPrefix(c.Text, goneDirective) {
			return nil, fmt.Errorf("%s is gone: write %s abc <reason>, if it is still too large", goneDirective, disableDirective)
		}
		if rest, found := strings.CutPrefix(c.Text, disableDirective+" "); found {
			return parseDisable(rest, known)
		}
	}
	return nil, nil
}

// parseDisable parses the metrics and the reason after the directive.
func parseDisable(rest string, known []string) ([]string, error) {
	list, reason, _ := strings.Cut(strings.TrimSpace(rest), " ")
	if strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("%s without a reason", disableDirective)
	}
	metrics := strings.Split(list, ",")
	for _, m := range metrics {
		if !slices.Contains(known, m) {
			return nil, fmt.Errorf("%s of %q: the metrics are %s", disableDirective, m, strings.Join(known, ","))
		}
	}
	return metrics, nil
}
