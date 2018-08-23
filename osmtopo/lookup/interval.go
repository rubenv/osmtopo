package lookup

import (
	"github.com/Workiva/go-datastructures/augmentedtree"
	"github.com/golang/geo/s2"
)

type interval struct {
	Cell  s2.CellID
	Loops []int64
}

func (s *interval) LowAtDimension(d uint64) int64 {
	return int64(s.Cell.RangeMin())
}

func (s *interval) HighAtDimension(d uint64) int64 {
	return int64(s.Cell.RangeMax())
}

func (s *interval) OverlapsAtDimension(i augmentedtree.Interval, d uint64) bool {
	return s.HighAtDimension(d) > i.LowAtDimension(d) &&
		s.LowAtDimension(d) < i.HighAtDimension(d)
}

func (s *interval) EqualAtDimension(i augmentedtree.Interval, d uint64) bool {
	return s.HighAtDimension(d) == i.LowAtDimension(d) &&
		s.LowAtDimension(d) == i.HighAtDimension(d)
}

func (s *interval) ID() uint64 {
	return uint64(s.Cell)
}
