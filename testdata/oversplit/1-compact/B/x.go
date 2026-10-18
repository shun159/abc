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
	for _, s := range cut.old {
		t.removeFile(s)
		t.log.segments.Add(-1)
	}
	t.removeSnapshotsBut(name)
	t.compactions.Add(1)
	t.lastCompact.Store(time.Now().UnixNano())
	return nil
}

// removeSnapshotsBut removes the snapshots of the table but the one of
// name.
func (t *Table[K, V]) removeSnapshotsBut(name string) {
	_, snaps, err := listLog(t.dir)
	if err != nil {
		return
	}
	for _, s := range snaps {
		if s.name != name {
			t.removeFile(s.name)
		}
	}
}

// removeFile removes the file of name from the directory of the table, and
// its size from the size on disk, if it was there.
func (t *Table[K, V]) removeFile(name string) {
	path := filepath.Join(t.dir, name)
	if fi, err := os.Stat(path); err == nil && os.Remove(path) == nil {
		t.log.disk.Add(-fi.Size())
	}
}
