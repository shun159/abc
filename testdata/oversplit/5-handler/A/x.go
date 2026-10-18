package observer

// Handler serves what n runs, for a look from outside, with curl say:
// /, /processes, /tree. ?format=json answers in JSON.
func Handler(n *proc.Node, apps *application.Running) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		st := n.Stats()
		trees := treesOf(r.Context(), n, apps)
		if asJSON(r) {
			reply(w, map[string]any{"stats": st, "trees": trees})
			return
		}
		fmt.Fprintf(w, "node %s: %d processes; %d spawned, %d exited, %d crashed\n\n",
			n.Name(), st.Processes, st.Spawned, st.Exited, st.Crashed)
		writeTrees(w, trees, r.URL.Query().Has("full"))
	})
	mux.HandleFunc("GET /processes", func(w http.ResponseWriter, r *http.Request) {
		infos := Processes(n)
		if asJSON(r) {
			reply(w, infos)
			return
		}
		WriteProcesses(w, infos)
	})
	mux.HandleFunc("GET /tree", func(w http.ResponseWriter, r *http.Request) {
		trees := treesOf(r.Context(), n, apps)
		if asJSON(r) {
			reply(w, trees)
			return
		}
		writeTrees(w, trees, r.URL.Query().Has("full"))
	})
	return mux
}
