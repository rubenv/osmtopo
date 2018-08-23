package lookup

import "github.com/golang/geo/s2"

type region struct {
	*s2.Loop
}

func (l *region) CapBound() s2.Cap {
	return l.Loop.CapBound()
}

func (l *region) ContainsCell(c s2.Cell) bool {
	for i := 0; i < 4; i++ {
		v := c.Vertex(i)
		if !l.ContainsPoint(v) {
			return false
		}
	}

	return true
}

func (l *region) IntersectsCell(c s2.Cell) bool {
	// if any of the cell's vertices is contained by the
	// loop they intersect
	for i := 0; i < 4; i++ {
		v := c.Vertex(i)
		if l.ContainsPoint(v) {
			return true
		}
	}

	// missing case from the above implementation
	// where the loop is fully contained by the cell
	for _, v := range l.Vertices() {
		if c.ContainsPoint(v) {
			return true
		}
	}

	return false
}
