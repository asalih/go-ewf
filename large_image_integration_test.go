package main

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asalih/go-ewf/evf1"
	"github.com/asalih/go-ewf/evf2"
)

// This test is intentionally skipped by default because it reads/writes ~10GB.
//
// Enable it locally with:
//
//	EWF_TESTDATA_DD=/path/to/your/image.001 go test -run TestLargeDDImageWriteRead ./...
//
func TestLargeDDImageWriteRead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large integration test in -short mode")
	}

	ddPath := os.Getenv("EWF_TESTDATA_DD")
	if ddPath == "" {
		t.Skip("set EWF_TESTDATA_DD=path/to/image.001 to run this large integration test")
		return
	}

	ddFile, err := os.Open(ddPath)
	if err != nil {
		t.Skipf("dd image not available (%s): %v", ddPath, err)
	}
	defer ddFile.Close()

	st, err := ddFile.Stat()
	if err != nil {
		t.Fatalf("stat dd image: %v", err)
	}
	ddSize := st.Size()
	if ddSize <= 0 {
		t.Fatalf("dd image size is invalid: %d", ddSize)
	}

	// Use deterministic sampling for stable failures.
	rnd := rand.New(rand.NewSource(1))

	// Create EVF2 and validate.
	t.Run("EVF2_CreateAndValidate", func(t *testing.T) {
		ewfPath := replaceExt(ddPath, ".Ex01")

		out, err := os.Create(ewfPath)
		if err != nil {
			t.Fatalf("create evf2 output: %v", err)
		}

		creator, err := evf2.CreateEWF(out)
		if err != nil {
			out.Close()
			t.Fatalf("evf2.CreateEWF: %v", err)
		}

		w, err := creator.Start(ddSize)
		if err != nil {
			out.Close()
			t.Fatalf("evf2.Start: %v", err)
		}

		start := time.Now()
		if _, err := io.CopyBuffer(w, ddFile, make([]byte, 4*1024*1024)); err != nil {
			out.Close()
			t.Fatalf("copy dd -> evf2: %v", err)
		}
		if err := w.Close(); err != nil {
			out.Close()
			t.Fatalf("close evf2 writer: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("close evf2 file: %v", err)
		}
		t.Logf("evf2 write completed in %s", time.Since(start))

		// Re-open for reading and validation.
		in, err := os.Open(ewfPath)
		if err != nil {
			t.Fatalf("open evf2 output: %v", err)
		}
		defer in.Close()

		reader, err := evf2.OpenEWF(in)
		if err != nil {
			t.Fatalf("evf2.OpenEWF: %v", err)
		}

		// Must have multiple tables for a 10GB image with 32KB chunks.
		if len(reader.First.Tables) < 2 {
			t.Fatalf("expected multiple tables, got %d", len(reader.First.Tables))
		}

		validateRandomWindows(t, ddFile, reader, ddSize, rnd)
	})

	// Reset dd file offset for subsequent runs (io.Copy advanced it).
	if _, err := ddFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek dd back to start: %v", err)
	}

	// EVF1 is optional because it is always compressed and will be slower; it still
	// validates our table base-offset logic on large outputs.
	if os.Getenv("EWF_LARGE_TEST_EVF1") == "" {
		t.Skip("set EWF_LARGE_TEST_EVF1=1 to also run EVF1 large create+validate")
	}

	t.Run("EVF1_CreateAndValidate", func(t *testing.T) {
		ewfPath := replaceExt(ddPath, ".E01")

		out, err := os.Create(ewfPath)
		if err != nil {
			t.Fatalf("create evf1 output: %v", err)
		}

		creator, err := evf1.CreateEWF(out)
		if err != nil {
			out.Close()
			t.Fatalf("evf1.CreateEWF: %v", err)
		}

		// Add minimal metadata to avoid empty identifier corner cases.
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_CASE_NUMBER, "LARGE-TEST")
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER, "LARGE-001")

		w, err := creator.Start()
		if err != nil {
			out.Close()
			t.Fatalf("evf1.Start: %v", err)
		}

		start := time.Now()
		if _, err := io.CopyBuffer(w, ddFile, make([]byte, 4*1024*1024)); err != nil {
			out.Close()
			t.Fatalf("copy dd -> evf1: %v", err)
		}
		if err := w.Close(); err != nil {
			out.Close()
			t.Fatalf("close evf1 writer: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("close evf1 file: %v", err)
		}
		t.Logf("evf1 write completed in %s", time.Since(start))

		in, err := os.Open(ewfPath)
		if err != nil {
			t.Fatalf("open evf1 output: %v", err)
		}
		defer in.Close()

		reader, err := evf1.OpenEWF(in)
		if err != nil {
			t.Fatalf("evf1.OpenEWF: %v", err)
		}

		if len(reader.First.Tables) < 2 {
			t.Fatalf("expected multiple tables in evf1 output, got %d", len(reader.First.Tables))
		}

		validateRandomWindows(t, ddFile, reader, ddSize, rnd)
	})
}

func replaceExt(path string, newExt string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + newExt
	}
	return strings.TrimSuffix(path, ext) + newExt
}

// validateRandomWindows compares multiple ReadAt windows between the dd file and EWF reader.
func validateRandomWindows(t *testing.T, ddFile *os.File, ewf io.ReaderAt, ddSize int64, rnd *rand.Rand) {
	t.Helper()

	type win struct {
		off int64
		sz  int
	}

	// A mix of fixed and random windows (keep total I/O reasonable).
	windows := []win{
		{off: 0, sz: 1 << 20},                    // start
		{off: 512, sz: 1 << 20},                  // sector boundary
		{off: 32768 - 512, sz: 1 << 20},          // near chunk boundary
		{off: 32768, sz: 1 << 20},                // chunk boundary
		{off: 2*32768 + 123, sz: 1 << 20},        // inside chunk
		{off: ddSize - (1 << 20), sz: 1 << 20},   // last 1MB
		{off: ddSize - (8 << 20), sz: 1 << 20},   // near end
		{off: ddSize/2 - (1 << 20), sz: 1 << 20}, // mid
		{off: ddSize/2 + (1 << 20), sz: 1 << 20}, // mid
		{off: ddSize/3 + 777, sz: 1 << 20},       // arbitrary
		{off: ddSize/4 + 1024, sz: 1 << 20},      // arbitrary
		{off: ddSize/5 + 4096, sz: 1 << 20},      // arbitrary
		{off: ddSize/6 + 8192, sz: 1 << 20},      // arbitrary
		{off: ddSize/7 + 16384, sz: 1 << 20},     // arbitrary
		{off: ddSize/8 + 32768, sz: 1 << 20},     // arbitrary
		{off: ddSize/9 + 65536, sz: 1 << 20},     // arbitrary
		{off: ddSize/10 + 131072, sz: 1 << 20},   // arbitrary
		{off: ddSize/11 + 262144, sz: 1 << 20},   // arbitrary
		{off: ddSize/12 + 524288, sz: 1 << 20},   // arbitrary
		{off: ddSize/13 + 1048576, sz: 1 << 20},  // arbitrary
		{off: ddSize/14 + 2097152, sz: 1 << 20},  // arbitrary
	}

	// Add random windows (64KB each).
	const randomN = 64
	const randomSz = 64 << 10
	if ddSize > randomSz {
		for i := 0; i < randomN; i++ {
			off := rnd.Int63n(ddSize - randomSz)
			windows = append(windows, win{off: off, sz: randomSz})
		}
	}

	for i, w := range windows {
		if w.off < 0 || w.off >= ddSize {
			continue
		}
		sz := w.sz
		if w.off+int64(sz) > ddSize {
			sz = int(ddSize - w.off)
		}
		if sz <= 0 {
			continue
		}

		ddBuf := make([]byte, sz)
		ewfBuf := make([]byte, sz)

		ndd, err := ddFile.ReadAt(ddBuf, w.off)
		if err != nil && err != io.EOF {
			t.Fatalf("dd ReadAt window %d offset=%d: %v", i, w.off, err)
		}
		newf, err := ewf.ReadAt(ewfBuf, w.off)
		if err != nil && err != io.EOF {
			t.Fatalf("ewf ReadAt window %d offset=%d: %v", i, w.off, err)
		}

		n := ndd
		if newf < n {
			n = newf
		}
		if n == 0 {
			continue
		}

		if !bytes.Equal(ddBuf[:n], ewfBuf[:n]) {
			// show a small diff location
			first := -1
			for j := 0; j < n; j++ {
				if ddBuf[j] != ewfBuf[j] {
					first = j
					break
				}
			}
			t.Fatalf("data mismatch at offset=%d (window %d), firstDiff=%d: dd=0x%02x ewf=0x%02x",
				w.off, i, first, ddBuf[first], ewfBuf[first])
		}
	}
}
