package osmtopo

import "github.com/paulsmith/gogeos/geos"

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
