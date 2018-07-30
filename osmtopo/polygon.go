package osmtopo

import (
	"github.com/Workiva/go-datastructures/augmentedtree"
	"github.com/golang/geo/s2"
	"github.com/paulsmith/gogeos/geos"
)

func MakePolygons(outerPolys, innerPolys []*geos.Geometry) (*geos.Geometry, error) {
	polygons := make([]*geos.Geometry, 0)
	for _, shell := range outerPolys {
		holes := make([][]geos.Coord, 0)

		if len(innerPolys) > 0 {
			pshell := geos.PrepareGeometry(shell)

			// Find holes
			for i := 0; i < len(innerPolys); i++ {
				hole := innerPolys[i]
				c, err := pshell.Contains(hole)
				if err != nil {
					return nil, err
				}
				if c {
					s, err := hole.Shell()
					if err != nil {
						return nil, err
					}

					c, err := s.Coords()
					if err != nil {
						return nil, err
					}

					holes = append(holes, c)
					innerPolys = append(innerPolys[:i], innerPolys[i+1:]...)
					i-- // Counter-act the increment at the end of the iteration
				}
			}
		}

		s, err := shell.Shell()
		if err != nil {
			return nil, err
		}

		scoords, err := s.Coords()
		if err != nil {
			return nil, err
		}

		polygon, err := geos.NewPolygon(scoords, holes...)
		if err != nil {
			return nil, err
		}

		size, err := polygon.Area()
		if err != nil {
			return nil, err
		}

		if size < 1e-5 {
			continue
		}

		polygons = append(polygons, polygon)
	}

	var feat *geos.Geometry
	if len(polygons) == 1 {
		feat = polygons[0]
	} else {
		f, err := geos.NewCollection(geos.MULTIPOLYGON, polygons...)
		if err != nil {
			return nil, err
		}
		feat = f
	}

	return feat, nil
}

func isClockwise(coords [][]float64) bool {
	sum := 0.0
	for i, coord := range coords[:len(coords)-1] {
		next := coords[i+1]
		sum += float64((next[0] - coord[0]) * (next[1] + coord[1]))
	}
	return sum >= 0
}

func reverse(coords [][]float64) [][]float64 {
	c := make([][]float64, len(coords))
	for i := 0; i < len(coords); i++ {
		c[i] = coords[len(coords)-i-1]
	}
	return c
}

func coordEquals(a, b []float64) bool {
	return a[0] == b[0] && a[1] == b[1]
}

func makeLoop(coords [][]float64) *s2.Loop {
	// s2.Loop is always CCW
	if isClockwise(coords) {
		coords = reverse(coords)
	}

	// Skip last point, not stored in loop
	points := make([]s2.Point, 0, len(coords)-1)
	for i := 0; i < len(coords)-1; i++ {
		if i > 0 && coordEquals(coords[i-1], coords[i]) {
			continue
		}
		latlon := s2.LatLngFromDegrees(coords[i][1], coords[i][0])
		points = append(points, s2.PointFromLatLng(latlon))
	}

	if len(points) < 3 {
		return nil
	}
	return s2.LoopFromPoints(points)
}

func hasDuplicates(coords [][]float64) bool {
	dupes := 0
	seen := make(map[[2]float64]bool)
	for _, point := range coords {
		p := [2]float64{point[0], point[1]}
		_, ok := seen[p]
		if ok {
			dupes += 1
		}
		seen[p] = true
	}
	return dupes > 1
}

type loopPolygon struct {
	outer *s2.Loop
	inner []*s2.Loop
}

func (l *loopPolygon) IsInside(lat, lng float64) bool {
	latlon := s2.LatLngFromDegrees(lat, lng)
	point := s2.PointFromLatLng(latlon)

	if !l.outer.ContainsPoint(point) {
		return false
	}

	for _, ring := range l.inner {
		if ring.ContainsPoint(point) {
			return false
		}
	}

	return true
}

type Region struct {
	*s2.Loop
}

func (l *Region) CapBound() s2.Cap {
	return l.Loop.CapBound()
}

func (l *Region) ContainsCell(c s2.Cell) bool {
	for i := 0; i < 4; i++ {
		v := c.Vertex(i)
		if !l.ContainsPoint(v) {
			return false
		}
	}

	return true
}

func (l *Region) IntersectsCell(c s2.Cell) bool {
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

type Interval struct {
	Cell  s2.CellID
	Loops []int64
}

func (s *Interval) LowAtDimension(d uint64) int64 {
	return int64(s.Cell.RangeMin())
}

func (s *Interval) HighAtDimension(d uint64) int64 {
	return int64(s.Cell.RangeMax())
}

func (s *Interval) OverlapsAtDimension(i augmentedtree.Interval, d uint64) bool {
	return s.HighAtDimension(d) > i.LowAtDimension(d) &&
		s.LowAtDimension(d) < i.HighAtDimension(d)
}

func (s *Interval) ID() uint64 {
	return uint64(s.Cell)
}
