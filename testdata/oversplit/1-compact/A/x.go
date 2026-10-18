package kvs

// compact snapshots the table as of a cut of the log, and removes the
// segments before the cut and the snapshots before this one.
func (t *Table[K, V]) compact() error {
	cut, err := t.cutLog()
	if err != nil {
		return err
	}
	name, err := t.snapshotAt(cut)
	if err != nil {
		return err
	}
	t.dropSegments(cut)
	t.dropSnapshots(name)
	t.countCompaction()
	return nil
}

// cutLog cuts the log.
func (t *Table[K, V]) cutLog() (logCut, error) {
	cut := t.log.cut()
	return cut, cut.err
}

// snapshotAt writes the snapshot of the cut, and returns its name.
func (t *Table[K, V]) snapshotAt(cut logCut) (string, error) {
	name := snapName(cut.seq)
	size, err := t.writeSnapshot(name, cut.seq)
	if err != nil {
		return "", err
	}
	t.addDisk(size)
	return name, nil
}

// addDisk adds n bytes to the size on disk.
func (t *Table[K, V]) addDisk(n int64) { t.log.disk.Add(n) }

// dropSegments removes the segments before the cut.
func (t *Table[K, V]) dropSegments(cut logCut) {
	for _, s := range cut.old {
		t.dropSegment(s)
	}
}

// dropSegment removes the segment of name.
func (t *Table[K, V]) dropSegment(name string) {
	t.removeFile(name)
	t.log.segments.Add(-1)
}

// dropSnapshots removes the snapshots but keep.
func (t *Table[K, V]) dropSnapshots(keep string) {
	_, snaps, err := listLog(t.dir)
	if err != nil {
		return
	}
	for _, s := range snaps {
		t.dropSnapshot(s.name, keep)
	}
}

// dropSnapshot removes the snapshot of name, but keep.
func (t *Table[K, V]) dropSnapshot(name, keep string) {
	if name != keep {
		t.removeFile(name)
	}
}

// removeFile removes the file of name, and its size from the size on disk.
func (t *Table[K, V]) removeFile(name string) {
	path := t.pathOf(name)
	if size, ok := sizeOf(path); ok && os.Remove(path) == nil {
		t.addDisk(-size)
	}
}

// pathOf is the path of the file of name.
func (t *Table[K, V]) pathOf(name string) string { return filepath.Join(t.dir, name) }

// sizeOf is the size of the file at path, if there is one.
func sizeOf(path string) (int64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	return fi.Size(), true
}

// countCompaction counts a compaction, done now.
func (t *Table[K, V]) countCompaction() {
	t.compactions.Add(1)
	t.lastCompact.Store(time.Now().UnixNano())
}
