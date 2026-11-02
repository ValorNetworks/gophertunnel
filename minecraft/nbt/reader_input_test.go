package nbt

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestDecoderAcceptsBufferReaderAndStream(t *testing.T) {
	want := map[string]any{"name": "velaris", "bytes": []byte{1, 2}, "ints": []int32{3, 4}, "longs": []int64{5, 6}}
	data, err := Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]io.Reader{
		"buffer": bytes.NewBuffer(data),
		"reader": bytes.NewReader(data),
		"stream": struct{ io.Reader }{bytes.NewReader(data)},
	} {
		t.Run(name, func(t *testing.T) {
			var got map[string]any
			if err := NewDecoder(input).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("decoded %v, want %v", got, want)
			}
		})
	}
}

func TestStreamingArrayLengthDoesNotAllocateAdvertisedSize(t *testing.T) {
	reader := newOffsetReader(struct{ io.Reader }{bytes.NewReader([]byte{1})})
	if _, err := reader.readArrayBytes(1<<30, 8, "test array"); err == nil {
		t.Fatal("truncated streaming array was accepted")
	}
	if reader.off != 1 {
		t.Fatalf("stream offset = %d, want 1", reader.off)
	}
}
