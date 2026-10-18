package observer

// Handler serves what n runs, for a look from outside, with curl say:
// /, /processes, /tree. ?format=json answers in JSON.
func Handler(n *proc.Node, apps *application.Running) http.Handler {
	v := view{n, apps}
	mux := http.NewServeMux()
	v.route(mux)
	return mux
}

// A view is what Handler serves: of n, and the trees of apps, if any.
type view struct {
	n    *proc.Node
	apps *application.Running
}

// route routes the pages of v on mux.
func (v view) route(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", v.root)
	mux.HandleFunc("GET /processes", v.processes)
	mux.HandleFunc("GET /tree", v.tree)
}

// root serves the counts of the processes, and the trees of apps.
func (v view) root(w http.ResponseWriter, r *http.Request) { v.serve(w, r, true) }

// tree serves the trees of apps.
func (v view) tree(w http.ResponseWriter, r *http.Request) { v.serve(w, r, false) }

// serve serves the trees of apps, with the counts of the processes if
// withStats.
func (v view) serve(w http.ResponseWriter, r *http.Request, withStats bool) {
	trees := v.trees(r)
	if asJSON(r) {
		v.replyJSON(w, trees, withStats)
		return
	}
	if withStats {
		v.writeStats(w)
	}
	writeTrees(w, trees, full(r))
}

// trees are the trees of apps.
func (v view) trees(r *http.Request) []appTree { return treesOf(r.Context(), v.n, v.apps) }

// replyJSON replies with the trees, and the counts if withStats, in JSON.
func (v view) replyJSON(w http.ResponseWriter, trees []appTree, withStats bool) {
	if withStats {
		reply(w, map[string]any{"stats": v.n.Stats(), "trees": trees})
		return
	}
	reply(w, trees)
}

// writeStats writes the counts of the processes.
func (v view) writeStats(w io.Writer) {
	st := v.n.Stats()
	fmt.Fprintf(w, "node %s: %d processes; %d spawned, %d exited, %d crashed\n\n",
		v.n.Name(), st.Processes, st.Spawned, st.Exited, st.Crashed)
}

// full tells whether r asks for the trees in full.
func full(r *http.Request) bool { return r.URL.Query().Has("full") }

// processes serves the processes, the most behind first.
func (v view) processes(w http.ResponseWriter, r *http.Request) {
	infos := Processes(v.n)
	if asJSON(r) {
		reply(w, infos)
		return
	}
	WriteProcesses(w, infos)
}
