package evf1

import "testing"

func TestEVF1TableBaseOffsetAnd31BitRelativeOffsets(t *testing.T) {
	seg, err := NewEWFSegment(nil)
	if err != nil {
		t.Fatalf("NewEWFSegment: %v", err)
	}
	seg.Tables = []*EWFTableSection{newTable()}

	// Pick offsets that force:
	// - BaseOffset initialization on first entry
	// - a 31-bit relative offset near the boundary on second entry
	// - a new table when relative offset would exceed 31 bits
	const base = int64(0x10)
	const nearLimit = int64(0x7FFFFFF0) // within 31-bit range if base is small
	const beyondLimit = int64(0x80000010)

	if err := seg.addTableEntry(base); err != nil {
		t.Fatalf("addTableEntry(base): %v", err)
	}
	if err := seg.addTableEntry(nearLimit); err != nil {
		t.Fatalf("addTableEntry(nearLimit): %v", err)
	}
	if err := seg.addTableEntry(beyondLimit); err != nil {
		t.Fatalf("addTableEntry(beyondLimit): %v", err)
	}

	if got := len(seg.Tables); got != 2 {
		t.Fatalf("expected 2 tables, got %d", got)
	}

	t0 := seg.Tables[0]
	if t0.Header.BaseOffset != uint64(base) {
		t.Fatalf("table0 BaseOffset mismatch: got %#x want %#x", t0.Header.BaseOffset, uint64(base))
	}
	if got := len(t0.Entries.Data); got != 2 {
		t.Fatalf("expected 2 entries in table0, got %d", got)
	}
	// entry0: rel=0, compressed flag set
	if (t0.Entries.Data[0]>>31) != 1 {
		t.Fatalf("table0 entry0 compressed flag not set: %#x", t0.Entries.Data[0])
	}
	if (t0.Entries.Data[0] & 0x7FFFFFFF) != 0 {
		t.Fatalf("table0 entry0 rel offset mismatch: got %#x want 0", t0.Entries.Data[0]&0x7FFFFFFF)
	}
	// entry1: rel = nearLimit - base
	wantRel1 := uint32(nearLimit - base)
	if (t0.Entries.Data[1]>>31) != 1 {
		t.Fatalf("table0 entry1 compressed flag not set: %#x", t0.Entries.Data[1])
	}
	if gotRel1 := (t0.Entries.Data[1] & 0x7FFFFFFF); gotRel1 != wantRel1 {
		t.Fatalf("table0 entry1 rel offset mismatch: got %#x want %#x", gotRel1, wantRel1)
	}

	t1 := seg.Tables[1]
	if t1.Header.BaseOffset != uint64(beyondLimit) {
		t.Fatalf("table1 BaseOffset mismatch: got %#x want %#x", t1.Header.BaseOffset, uint64(beyondLimit))
	}
	if got := len(t1.Entries.Data); got != 1 {
		t.Fatalf("expected 1 entry in table1, got %d", got)
	}
	if (t1.Entries.Data[0]>>31) != 1 {
		t.Fatalf("table1 entry0 compressed flag not set: %#x", t1.Entries.Data[0])
	}
	if (t1.Entries.Data[0] & 0x7FFFFFFF) != 0 {
		t.Fatalf("table1 entry0 rel offset mismatch: got %#x want 0", t1.Entries.Data[0]&0x7FFFFFFF)
	}
}

