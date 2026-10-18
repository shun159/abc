package kvs

// compact snapshots the table as of a cut of the log, and removes the
// segments before the cut and the snapshots before this one.
func (t *Table[K, V]) compact() error {
	cut := t.log.cut()
	if cut.err != nil {
		return cut.err
	}
	name := snapName(cut.seq)
	size, err := t.writeSnapshot(name, cut.seq)
	if err != nil {
		return err
	}
	t.log.disk.Add(size)
	remove := func(name string) {
		path := filepath.Join(t.dir, name)
		if fi, err := os.Stat(path); err == nil && os.Remove(path) == nil {
			t.log.disk.Add(-fi.Size())
		}
	}
	for _, s := range cut.old {
		remove(s)
		t.log.segments.Add(-1)
	}
	if _, snaps, err := listLog(t.dir); err == nil {
		for _, s := range snaps {
			if s.name != name {
				remove(s.name)
			}
		}
	}
	t.compactions.Add(1)
	t.lastCompact.Store(time.Now().UnixNano())
	return nil
}
