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
}
