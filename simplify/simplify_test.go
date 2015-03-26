package simplify

import (
	"reflect"
	"testing"
)

func TestSingleCoordNOOP(t *testing.T) {
	segment := []int64{1}
	segments := [][]int64{segment}
	if !reflect.DeepEqual(Reduce(segments), segments) {
		t.Fatal("Should be a NOOP")
	}
}

func TestMergesLines(t *testing.T) {
	input := [][]int64{
		[]int64{1, 2},
		[]int64{2, 3},
	}
	expected := [][]int64{
		[]int64{1, 2, 3},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func TestPreserveBodies(t *testing.T) {
	input := [][]int64{
		[]int64{1, 2, 3},
		[]int64{3, 4, 5},
	}
	expected := [][]int64{
		[]int64{1, 2, 3, 4, 5},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func TestMergeMultiple(t *testing.T) {
	input := [][]int64{
		[]int64{1, 2},
		[]int64{2, 3},
		[]int64{3, 4},
	}
	expected := [][]int64{
		[]int64{1, 2, 3, 4},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func TestMergeOrder(t *testing.T) {
	input := [][]int64{
		[]int64{2, 3},
		[]int64{3, 4},
		[]int64{1, 2},
	}
	expected := [][]int64{
		[]int64{1, 2, 3, 4},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func TestMergeCircular(t *testing.T) {
	input := [][]int64{
		[]int64{1, 2},
		[]int64{2, 3},
		[]int64{3, 1},
	}
	expected := [][]int64{
		[]int64{1, 2, 3, 1},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func TestInverted(t *testing.T) {
	input := [][]int64{
		[]int64{1, 2},
		[]int64{3, 2},
		[]int64{3, 4},
	}
	expected := [][]int64{
		[]int64{1, 2, 3, 4},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func TestInvertedBodies(t *testing.T) {
	input := [][]int64{
		[]int64{1, 2, 3},
		[]int64{5, 4, 3},
		[]int64{5, 6, 7},
	}
	expected := [][]int64{
		[]int64{1, 2, 3, 4, 5, 6, 7},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func TestSeparate(t *testing.T) {
	input := [][]int64{
		[]int64{1, 2},
		[]int64{2, 3},
		[]int64{4, 5},
		[]int64{5, 6},
	}
	expected := [][]int64{
		[]int64{1, 2, 3},
		[]int64{4, 5, 6},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func TestStart(t *testing.T) {
	input := [][]int64{
		[]int64{1, 2, 3},
		[]int64{1, 4, 5},
	}
	expected := [][]int64{
		[]int64{5, 4, 1, 2, 3},
	}
	if !reflect.DeepEqual(Reduce(input), expected) {
		t.Fatal("Failed")
	}
}

func BenchmarkSimplify(b *testing.B) {
	input := [][]int64{
		[]int64{1, 2, 3},
		[]int64{3, 4, 5},
	}
	for n := 0; n < b.N; n++ {
		Reduce(input)
	}
}
