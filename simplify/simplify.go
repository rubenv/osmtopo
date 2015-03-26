package simplify

func reverse(s []int64) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func Reduce(in [][]int64) [][]int64 {
	// Keep optimizing until we can't find anything left to optimize.
	repeat := true
	for repeat {
		repeat = false

		// Iterate over each piece, try to match the end of a line
		// with the start of other lines.
		length := len(in)
		for i := 0; i < length; i++ {
			line := in[i]
			start := line[0]
			end := line[len(line)-1]

			for j := 0; j < length; j++ {
				line2 := in[j]
				start2 := line2[0]
				end2 := line2[len(line2)-1]

				if i == j {
					continue
				}

				if end == start2 {
					rest := line2[1:]
					in[i] = append(in[i], rest...)
					in = append(in[:j], in[j+1:]...)
					repeat = true
					break
				}

				// Same end? Append reversed
				if end2 == end {
					reverse(line2)
					in[i] = append(in[i], line2[1:]...)
					in = append(in[:j], in[j+1:]...)
					repeat = true
					break
				}

				// Same start? Prepend!
				if start2 == start {
					reverse(line2)
					in[i] = append(line2[0:len(line2)-1], in[i]...)
					in = append(in[:j], in[j+1:]...)
					repeat = true
				}
			}

			// Need to restart the iteration, break out of current loop
			if repeat {
				break
			}
		}
	}
	return in
}
