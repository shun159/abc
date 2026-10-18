package observer

// Handler serves what n runs, for a look from outside, with curl say:
// /, /processes, /tree. ?format=json answers in JSON.
func Handler(n *proc.Node, apps *application.Running) http.Handler {
	v := view{n, apps}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", v.root)
	mux.HandleFunc("GET /processes", v.processes)
	mux.HandleFunc("GET /tree", v.tree)
	return mux
}

// A view is what Handler serves: of n, and the trees of apps, if any.
type view struct {
	n    *proc.Node
	apps *application.Running
}

// root serves the counts of the processes, and the trees of apps.
func (v view) root(w http.ResponseWriter, r *http.Request) {
	st := v.n.Stats()
	trees := treesOf(r.Context(), v.n, v.apps)
	if asJSON(r) {
		reply(w, map[string]any{"stats": st, "trees": trees})
		return
	}
	fmt.Fprintf(w, "node %s: %d processes; %d spawned, %d exited, %d crashed\n\n",
		v.n.Name(), st.Processes, st.Spawned, st.Exited, st.Crashed)
	writeTrees(w, trees, r.URL.Query().Has("full"))
}

// processes serves the processes, the most behind first.
func (v view) processes(w http.ResponseWriter, r *http.Request) {
	infos := Processes(v.n)
	if asJSON(r) {
		reply(w, infos)
		return
	}
	WriteProcesses(w, infos)
}

// tree serves the trees of apps.
func (v view) tree(w http.ResponseWriter, r *http.Request) {
	trees := treesOf(r.Context(), v.n, v.apps)
	if asJSON(r) {
		reply(w, trees)
		return
	}
	writeTrees(w, trees, r.URL.Query().Has("full"))
}
