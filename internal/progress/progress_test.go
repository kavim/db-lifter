package progress

import (
	"io"
	"strings"
	"testing"
)

func TestReader_CountsBytes(t *testing.T) {
	const payload = "hello world"
	r := NewReader(strings.NewReader(payload), int64(len(payload)))
	buf := make([]byte, 4)
	var total int
	for {
		n, err := r.Read(buf)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if total != len(payload) {
		t.Fatalf("bytes read = %d, want %d", total, len(payload))
	}
	if r.BytesRead() != int64(len(payload)) {
		t.Fatalf("BytesRead() = %d", r.BytesRead())
	}
	if got, want := r.Percent(), 1.0; got != want {
		t.Fatalf("Percent() = %v, want %v", got, want)
	}
}

func TestReader_Indeterminate(t *testing.T) {
	r := NewReader(strings.NewReader("x"), -1)
	if !r.Indeterminate() {
		t.Fatal("expected indeterminate")
	}
	if r.Percent() != 0 {
		t.Fatalf("Percent() = %v", r.Percent())
	}
}

func TestReader_PercentCapsAtOne(t *testing.T) {
	r := NewReader(strings.NewReader("abcd"), 2)
	buf := make([]byte, 10)
	if _, err := io.CopyBuffer(io.Discard, r, buf); err != nil {
		t.Fatal(err)
	}
	if r.Percent() != 1.0 {
		t.Fatalf("Percent() = %v, want 1.0", r.Percent())
	}
}
