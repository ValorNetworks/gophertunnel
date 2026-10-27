package nbt

import (
	"bytes"
	"math"
	"testing"
)

func TestFixedWidthArrayRejectsInvalidLengthBeforeAllocation(t *testing.T) {
	for _, count := range []int32{-1, 1, math.MaxInt32} {
		for _, width := range []int{4, 8} {
			reader := newOffsetReader(bytes.NewBuffer([]byte{1, 2, 3}))
			if _, err := reader.readArrayBytes(count, width, "test"); err == nil {
				t.Fatalf("accepted count=%d width=%d with three bytes", count, width)
			}
			if reader.off != 0 {
				t.Fatalf("invalid length consumed %d bytes", reader.off)
			}
		}
	}
}

func TestFixedWidthArrayReadsExactBytes(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	reader := newOffsetReader(bytes.NewBuffer(data))
	got, err := reader.readArrayBytes(2, 4, "test")
	if err != nil || !bytes.Equal(got, data[:8]) || reader.off != 8 || reader.Reader.Len() != 1 {
		t.Fatalf("read = %x, %v; offset=%d remaining=%d", got, err, reader.off, reader.Reader.Len())
	}
}
