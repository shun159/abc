package kvs

// grow gives child i of n more than minItems entries: one from a sibling,
// through n, or all of a sibling, merged with it.
func (n *bnode[K, V]) grow(i int, gen uint64) {
	switch {
	case i > 0 && len(n.kids[i-1].items) > minItems:
		n.takeLeft(i, gen)
	case i < len(n.items) && len(n.kids[i+1].items) > minItems:
		n.takeRight(i, gen)
	case i >= len(n.items):
		n.merge(i-1, gen)
	default:
		n.merge(i, gen)
	}
}

// takeLeft gives child i of n the last entry of its left sibling, through
// n: the entry of n between them goes down to it, the sibling's up in
// its place; and the sibling's last child, if any.
func (n *bnode[K, V]) takeLeft(i int, gen uint64) {
	c, left := n.kid(i, gen), n.kid(i-1, gen)
	c.items = slices.Insert(c.items, 0, n.items[i-1])
	last := len(left.items) - 1
	n.items[i-1] = left.items[last]
	left.items[last] = leaf[K, V]{}
	left.items = left.items[:last]
	if len(left.kids) > 0 {
		last := len(left.kids) - 1
		c.kids = slices.Insert(c.kids, 0, left.kids[last])
		left.kids[last] = nil
		left.kids = left.kids[:last]
	}
}

// takeRight gives child i of n the first entry of its right sibling, as
// takeLeft does the last of its left.
func (n *bnode[K, V]) takeRight(i int, gen uint64) {
	c, right := n.kid(i, gen), n.kid(i+1, gen)
	c.items = append(c.items, n.items[i])
	n.items[i] = right.items[0]
	right.items = slices.Delete(right.items, 0, 1)
	if len(right.kids) > 0 {
		c.kids = append(c.kids, right.kids[0])
		right.kids = slices.Delete(right.kids, 0, 1)
	}
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
