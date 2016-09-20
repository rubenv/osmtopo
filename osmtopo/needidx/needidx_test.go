package needidx

import (
	"testing"

	"github.com/cheekybits/is"
)

func TestNeedIdx(t *testing.T) {
	is := is.New(t)

	idx := New()
	is.NotNil(idx)

	is.False(idx.IsNeeded(123))
	is.False(idx.IsNeeded(124))

	idx.MarkNeeded(123)
	is.True(idx.IsNeeded(123))
	is.False(idx.IsNeeded(124))

	idx.MarkNeeded(1025)
	is.True(idx.IsNeeded(1025))
	is.False(idx.IsNeeded(1024))

	idx.MarkNeeded(10250)
	is.True(idx.IsNeeded(10250))
	is.False(idx.IsNeeded(10240))

	is.False(idx.IsNeeded(102400))
	is.False(idx.IsNeeded(1024000))
	is.False(idx.IsNeeded(10240000))

	for i := 0; i < 8; i++ {
		is.False(idx.IsNeeded(1 >> uint32(i)))
	}

	for i := 0; i < 8; i++ {
		idx.MarkNeeded(1 >> uint32(i))
	}

	for i := 0; i < 8; i++ {
		is.True(idx.IsNeeded(1 >> uint32(i)))
	}
}
