package kvs

// grow gives child i of n more than minItems entries: one from a sibling,
// through n, or all of a sibling, merged with it.
func (n *bnode[K, V]) grow(i int, gen uint64) {
	switch {
	case n.leftHasSpare(i):
		n.takeLeft(i, gen)
	case n.rightHasSpare(i):
		n.takeRight(i, gen)
	default:
		n.merge(n.mergeAt(i), gen)
	}
}

// leftHasSpare tells whether the left sibling of child i has an entry to
// spare.
func (n *bnode[K, V]) leftHasSpare(i int) bool { return i > 0 && len(n.kids[i-1].items) > minItems }

// rightHasSpare tells whether the right sibling of child i has an entry
// to spare.
func (n *bnode[K, V]) rightHasSpare(i int) bool {
	return i < len(n.items) && len(n.kids[i+1].items) > minItems
}

// mergeAt is the child to merge its right sibling into, for child i.
func (n *bnode[K, V]) mergeAt(i int) int {
	if i >= len(n.items) {
		return i - 1
	}
	return i
}

// takeLeft gives child i of n the last entry of its left sibling.
func (n *bnode[K, V]) takeLeft(i int, gen uint64) {
	c, left := n.kid(i, gen), n.kid(i-1, gen)
	c.items, n.items[i-1], left.items = rotateRight(c.items, n.items[i-1], left.items)
	c.kids, left.kids = moveLastKid(c.kids, left.kids)
}

// rotateRight moves sep to the front of to, and the last of from in place
// of sep.
func rotateRight[K comparable, V any](to []leaf[K, V], sep leaf[K, V], from []leaf[K, V]) ([]leaf[K, V], leaf[K, V], []leaf[K, V]) {
	to = slices.Insert(to, 0, sep)
	last := len(from) - 1
	sep = from[last]
	from[last] = leaf[K, V]{}
	return to, sep, from[:last]
}

// moveLastKid moves the last kid of from to the front of to, if any.
func moveLastKid[K comparable, V any](to, from []*bnode[K, V]) ([]*bnode[K, V], []*bnode[K, V]) {
	if len(from) == 0 {
		return to, from
	}
	last := len(from) - 1
	to = slices.Insert(to, 0, from[last])
	from[last] = nil
	return to, from[:last]
}

// takeRight gives child i of n the first entry of its right sibling.
func (n *bnode[K, V]) takeRight(i int, gen uint64) {
	c, right := n.kid(i, gen), n.kid(i+1, gen)
	c.items, n.items[i], right.items = rotateLeft(c.items, n.items[i], right.items)
	c.kids, right.kids = moveFirstKid(c.kids, right.kids)
}

// rotateLeft moves sep to the end of to, and the first of from in place
// of sep.
func rotateLeft[K comparable, V any](to []leaf[K, V], sep leaf[K, V], from []leaf[K, V]) ([]leaf[K, V], leaf[K, V], []leaf[K, V]) {
	to = append(to, sep)
	sep = from[0]
	return to, sep, slices.Delete(from, 0, 1)
}

// moveFirstKid moves the first kid of from to the end of to, if any.
func moveFirstKid[K comparable, V any](to, from []*bnode[K, V]) ([]*bnode[K, V], []*bnode[K, V]) {
	if len(from) == 0 {
		return to, from
	}
	return append(to, from[0]), slices.Delete(from, 0, 1)
}

// merge merges child i+1 of n into child i, with the entry of n between
// them.
func (n *bnode[K, V]) merge(i int, gen uint64) {
	c, right := n.kid(i, gen), n.kids[i+1]
	c.items = append(c.items, n.items[i])
	c.items = append(c.items, right.items...)
	c.kids = append(c.kids, right.kids...)
	n.items = slices.Delete(n.items, i, i+1)
	n.kids = slices.Delete(n.kids, i+1, i+2)
}
