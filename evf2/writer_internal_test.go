package evf2

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestEVF2WriterSplitsTablesAndReadsAcrossBoundary(t *testing.T) {
	// Lower threshold so the test triggers a split quickly.
	old := maxTableLength
	maxTableLength = 2
	defer func() { maxTableLength = old }()

	// 3 full chunks -> should produce 2 tables (2 entries + 1 entry).
	data := make([]byte, 3*DefaultChunkSize)
	for i := range data {
		data[i] = byte((i * 131) % 251) // deterministic pattern
	}

	tmpDir := t.TempDir()
	ewfPath := filepath.Join(tmpDir, "split.Ex01")

	f, err := os.Create(ewfPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	creator, err := CreateEWF(f)
	if err != nil {
		t.Fatalf("CreateEWF: %v", err)
	}
	w, err := creator.Start(int64(len(data)))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}

	// Read back and ensure boundary correctness.
	rf, err := os.Open(ewfPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rf.Close()

	reader, err := OpenEWF(rf)
	if err != nil {
		t.Fatalf("OpenEWF: %v", err)
	}

	if got := len(reader.First.Tables); got != 2 {
		t.Fatalf("expected 2 tables, got %d", got)
	}
	if reader.First.Tables[0].Header.FirstChunkNumber != 0 {
		t.Fatalf("table0 FirstChunkNumber mismatch: got %d want 0", reader.First.Tables[0].Header.FirstChunkNumber)
	}
	if reader.First.Tables[1].Header.FirstChunkNumber != 2 {
		t.Fatalf("table1 FirstChunkNumber mismatch: got %d want 2", reader.First.Tables[1].Header.FirstChunkNumber)
	}

	readAll := make([]byte, len(data))
	if _, err := io.ReadFull(reader, readAll); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if !bytes.Equal(data, readAll) {
		t.Fatalf("data mismatch after full read")
	}

	// Random read that starts in the 3rd chunk (table1).
	off := int64(2*DefaultChunkSize + 123)
	buf := make([]byte, 4096)
	n, err := reader.ReadAt(buf, off)
	if err != nil && err != io.EOF {
		t.Fatalf("ReadAt: %v", err)
	}
	if !bytes.Equal(buf[:n], data[off:off+int64(n)]) {
		t.Fatalf("data mismatch after ReadAt at offset %d", off)
	}
}

