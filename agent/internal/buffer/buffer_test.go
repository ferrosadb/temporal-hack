package buffer

import (
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Buffer {
	t.Helper()
	p := filepath.Join(t.TempDir(), "buf.db")
	b, err := Open(p)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestAppendPeekAck(t *testing.T) {
	b := openTemp(t)
	for i := 0; i < 5; i++ {
		if err := b.Append(Sample{RobotID: "r1", Stream: "battery", Payload: []byte{byte(i)}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := b.Peek(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("want 5, got %d", len(got))
	}
	ids := []int64{got[0].ID, got[1].ID}
	if err := b.Ack(ids); err != nil {
		t.Fatal(err)
	}
	n, _ := b.Count()
	if n != 3 {
		t.Fatalf("want 3 after ack, got %d", n)
	}
}

func TestDropOldestEviction(t *testing.T) {
	b := openTemp(t)
	b.maxSamples = 3
	for i := 0; i < 5; i++ {
		if err := b.Append(Sample{RobotID: "r1", Stream: "x", Payload: []byte{byte(i)}}); err != nil {
			t.Fatal(err)
		}
	}
	n, _ := b.Count()
	if n != 3 {
		t.Fatalf("want 3 after eviction, got %d", n)
	}
	got, _ := b.Peek(10)
	if got[0].Payload[0] != 2 {
		t.Fatalf("oldest 2 should remain at head; got %v", got[0].Payload)
	}
}
