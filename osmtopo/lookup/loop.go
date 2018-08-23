package lookup

import "github.com/golang/geo/s2"

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
