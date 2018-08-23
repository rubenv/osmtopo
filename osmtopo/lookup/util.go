package lookup

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
