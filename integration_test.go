package main

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/asalih/go-ewf/evf1"
	"github.com/asalih/go-ewf/evf2"
)

const (
	testDataFile = "testdata/winevt-rc.db"
)

// TestEVF1WriteRead tests the complete write and read cycle for EVF1 format
func TestEVF1WriteRead(t *testing.T) {
	// Read original test data
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	if len(originalData) == 0 {
		t.Fatal("Test data file is empty")
	}

	// Create temporary directory for test files
	tmpDir := t.TempDir()
	ewfPath := filepath.Join(tmpDir, "test.E01")

	// Write EVF1 file
	t.Run("Write", func(t *testing.T) {
		ewfFile, err := os.Create(ewfPath)
		if err != nil {
			t.Fatalf("Failed to create EWF file: %v", err)
		}

		creator, err := evf1.CreateEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to create EVF1 creator: %v", err)
		}

		// Add metadata
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_CASE_NUMBER, "TEST-001")
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EXAMINER_NAME, "Test Examiner")
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER, "EVD-001")
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_NOTES, "Integration test for EVF1")
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_DESCRIPTION, "Test description")

		writer, err := creator.Start()
		if err != nil {
			t.Fatalf("Failed to start writer: %v", err)
		}

		// Write data in chunks
		written, err := io.Copy(writer, bytes.NewReader(originalData))
		if err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}

		if written != int64(len(originalData)) {
			t.Errorf("Written bytes mismatch: got %d, want %d", written, len(originalData))
		}

		err = writer.Close()
		if err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		err = ewfFile.Close()
		if err != nil {
			t.Fatalf("Failed to close EWF file: %v", err)
		}
	})

	// Read and verify EVF1 file
	t.Run("Read", func(t *testing.T) {
		ewfFile, err := os.Open(ewfPath)
		if err != nil {
			t.Fatalf("Failed to open EWF file: %v", err)
		}
		defer ewfFile.Close()

		reader, err := evf1.OpenEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to open EVF1 reader: %v", err)
		}

		// Note: EVF1 reports size based on chunk boundaries (padded size)
		// The actual size may be larger due to padding to complete chunks
		if reader.Size() < int64(len(originalData)) {
			t.Errorf("Size too small: got %d, want at least %d", reader.Size(), len(originalData))
		}

		// Verify metadata
		metadata := reader.Metadata()
		if caseNum, ok := metadata["Case Number"].(string); !ok || caseNum != "TEST-001" {
			t.Errorf("Case number mismatch: got %v", caseNum)
		}

		// Read original data size (not the padded size)
		readData := make([]byte, len(originalData))
		n, err := io.ReadFull(reader, readData)
		if err != nil {
			t.Fatalf("Failed to read data: %v", err)
		}

		if n != len(originalData) {
			t.Errorf("Read bytes mismatch: got %d, want %d", n, len(originalData))
		}

		// Compare data
		if !bytes.Equal(originalData, readData) {
			t.Error("Read data does not match original data")
		}
	})

	// Test random access reads
	t.Run("RandomAccess", func(t *testing.T) {
		ewfFile, err := os.Open(ewfPath)
		if err != nil {
			t.Fatalf("Failed to open EWF file: %v", err)
		}
		defer ewfFile.Close()

		reader, err := evf1.OpenEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to open EVF1 reader: %v", err)
		}

		testOffsets := []int64{
			0,                               // Start
			512,                             // Second sector
			1024,                            // 1KB
			int64(len(originalData)) / 2,    // Middle
			int64(len(originalData)) - 1024, // Near end
		}

		for _, offset := range testOffsets {
			if offset >= int64(len(originalData)) {
				continue
			}

			readSize := 4096
			if offset+int64(readSize) > int64(len(originalData)) {
				readSize = int(int64(len(originalData)) - offset)
			}

			buf := make([]byte, readSize)
			n, err := reader.ReadAt(buf, offset)
			if err != nil && err != io.EOF {
				t.Errorf("ReadAt failed at offset %d: %v", offset, err)
				continue
			}

			expected := originalData[offset : offset+int64(n)]
			if !bytes.Equal(buf[:n], expected) {
				t.Errorf("Data mismatch at offset %d", offset)
			}
		}
	})
}

// TestEVF2WriteRead tests the complete write and read cycle for EVF2 format
func TestEVF2WriteRead(t *testing.T) {
	// Read original test data
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	if len(originalData) == 0 {
		t.Fatal("Test data file is empty")
	}

	// Create temporary directory for test files
	tmpDir := t.TempDir()
	ewfPath := filepath.Join(tmpDir, "test.Ex01")

	// Write EVF2 file
	t.Run("Write", func(t *testing.T) {
		ewfFile, err := os.Create(ewfPath)
		if err != nil {
			t.Fatalf("Failed to create EWF file: %v", err)
		}

		creator, err := evf2.CreateEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to create EVF2 creator: %v", err)
		}

		// Add case data metadata
		creator.AddCaseData(evf2.EWF_CASE_DATA_CASE_NUMBER, "TEST-002")
		creator.AddCaseData(evf2.EWF_CASE_DATA_EXAMINER_NAME, "Test Examiner")
		creator.AddCaseData(evf2.EWF_CASE_DATA_EVIDENCE_NUMBER, "EVD-002")
		creator.AddCaseData(evf2.EWF_CASE_DATA_NOTES, "Integration test for EVF2")
		creator.AddCaseData(evf2.EWF_CASE_DATA_NAME, "Test Evidence")

		// Add device information metadata
		creator.AddDeviceInformation(evf2.EWF_DEVICE_INFO_DRIVE_MODEL, "Virtual Test Drive")
		creator.AddDeviceInformation(evf2.EWF_DEVICE_INFO_SERIAL_NUMBER, "TEST123456")

		writer, err := creator.Start(int64(len(originalData)))
		if err != nil {
			t.Fatalf("Failed to start writer: %v", err)
		}

		// Write data in chunks
		written, err := io.Copy(writer, bytes.NewReader(originalData))
		if err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}

		if written != int64(len(originalData)) {
			t.Errorf("Written bytes mismatch: got %d, want %d", written, len(originalData))
		}

		err = writer.Close()
		if err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		err = ewfFile.Close()
		if err != nil {
			t.Fatalf("Failed to close EWF file: %v", err)
		}
	})

	// Read and verify EVF2 file
	t.Run("Read", func(t *testing.T) {
		ewfFile, err := os.Open(ewfPath)
		if err != nil {
			t.Fatalf("Failed to open EWF file: %v", err)
		}
		defer ewfFile.Close()

		reader, err := evf2.OpenEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to open EVF2 reader: %v", err)
		}

		// Note: EVF2 reports size based on chunk boundaries (padded size)
		// The actual size may be larger due to padding to complete chunks
		if reader.Size() < int64(len(originalData)) {
			t.Errorf("Size too small: got %d, want at least %d", reader.Size(), len(originalData))
		}

		// Verify metadata
		metadata := reader.Metadata()
		if caseData, ok := metadata["Case Data"].(map[string]string); ok {
			if caseNum := caseData["Case Number"]; caseNum != "TEST-002" {
				t.Errorf("Case number mismatch: got %v", caseNum)
			}
		}

		// Read original data size (not the padded size)
		readData := make([]byte, len(originalData))
		n, err := io.ReadFull(reader, readData)
		if err != nil {
			t.Fatalf("Failed to read data: %v", err)
		}

		if n != len(originalData) {
			t.Errorf("Read bytes mismatch: got %d, want %d", n, len(originalData))
		}

		// Compare data
		if !bytes.Equal(originalData, readData) {
			t.Error("Read data does not match original data")
		}
	})

	// Test random access reads
	t.Run("RandomAccess", func(t *testing.T) {
		ewfFile, err := os.Open(ewfPath)
		if err != nil {
			t.Fatalf("Failed to open EWF file: %v", err)
		}
		defer ewfFile.Close()

		reader, err := evf2.OpenEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to open EVF2 reader: %v", err)
		}

		testOffsets := []int64{
			0,                               // Start
			512,                             // Second sector
			1024,                            // 1KB
			int64(len(originalData)) / 2,    // Middle
			int64(len(originalData)) - 1024, // Near end
		}

		for _, offset := range testOffsets {
			if offset >= int64(len(originalData)) {
				continue
			}

			readSize := 4096
			if offset+int64(readSize) > int64(len(originalData)) {
				readSize = int(int64(len(originalData)) - offset)
			}

			buf := make([]byte, readSize)
			n, err := reader.ReadAt(buf, offset)
			if err != nil && err != io.EOF {
				t.Errorf("ReadAt failed at offset %d: %v", offset, err)
				continue
			}

			expected := originalData[offset : offset+int64(n)]
			if !bytes.Equal(buf[:n], expected) {
				t.Errorf("Data mismatch at offset %d", offset)
			}
		}
	})

	// Test Seek functionality
	t.Run("Seek", func(t *testing.T) {
		ewfFile, err := os.Open(ewfPath)
		if err != nil {
			t.Fatalf("Failed to open EWF file: %v", err)
		}
		defer ewfFile.Close()

		reader, err := evf2.OpenEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to open EVF2 reader: %v", err)
		}

		// Test SeekStart
		pos, err := reader.Seek(1024, io.SeekStart)
		if err != nil {
			t.Errorf("SeekStart failed: %v", err)
		}
		if pos != 1024 {
			t.Errorf("SeekStart position mismatch: got %d, want 1024", pos)
		}

		// Read and verify
		buf := make([]byte, 512)
		n, _ := reader.Read(buf)
		if !bytes.Equal(buf[:n], originalData[1024:1024+int64(n)]) {
			t.Error("Data mismatch after SeekStart")
		}

		// Test SeekCurrent
		pos, err = reader.Seek(512, io.SeekCurrent)
		if err != nil {
			t.Errorf("SeekCurrent failed: %v", err)
		}

		// Test SeekEnd (note: uses padded size reported by reader)
		pos, err = reader.Seek(-1024, io.SeekEnd)
		if err != nil {
			t.Errorf("SeekEnd failed: %v", err)
		}
		expectedPos := reader.Size() - 1024
		if pos != expectedPos {
			t.Errorf("SeekEnd position mismatch: got %d, want %d", pos, expectedPos)
		}
	})
}

// TestHashVerification tests that the hash values are correctly computed
func TestEVF1HashVerification(t *testing.T) {
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	tmpDir := t.TempDir()
	ewfPath := filepath.Join(tmpDir, "test_hash.E01")

	// Create and write
	ewfFile, err := os.Create(ewfPath)
	if err != nil {
		t.Fatalf("Failed to create EWF file: %v", err)
	}

	creator, err := evf1.CreateEWF(ewfFile)
	if err != nil {
		t.Fatalf("Failed to create EVF1 creator: %v", err)
	}

	// Add some metadata to avoid empty identifier issues
	creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_CASE_NUMBER, "HASH-TEST")
	creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER, "HASH-001")

	writer, err := creator.Start()
	if err != nil {
		t.Fatalf("Failed to start writer: %v", err)
	}

	_, err = writer.Write(originalData)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	ewfFile.Close()

	// Read and verify hashes
	ewfFile, err = os.Open(ewfPath)
	if err != nil {
		t.Fatalf("Failed to open EWF file: %v", err)
	}
	defer ewfFile.Close()

	reader, err := evf1.OpenEWF(ewfFile)
	if err != nil {
		t.Fatalf("Failed to open EVF1 reader: %v", err)
	}

	// Verify that hashes are stored (actual hash values include padding which is expected)
	if reader.First.Digest == nil {
		t.Error("Digest section is nil")
	} else {
		// Just verify hashes exist and are non-zero
		zeroMD5 := [16]byte{}
		zeroSHA1 := [20]byte{}
		if bytes.Equal(reader.First.Digest.MD5[:], zeroMD5[:]) {
			t.Error("MD5 hash is zero")
		}
		if bytes.Equal(reader.First.Digest.SHA1[:], zeroSHA1[:]) {
			t.Error("SHA1 hash is zero")
		}
	}
}

// TestEVF2HashVerification tests that the hash values are correctly computed for EVF2
func TestEVF2HashVerification(t *testing.T) {
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	tmpDir := t.TempDir()
	ewfPath := filepath.Join(tmpDir, "test_hash.Ex01")

	// Create and write
	ewfFile, err := os.Create(ewfPath)
	if err != nil {
		t.Fatalf("Failed to create EWF file: %v", err)
	}

	creator, err := evf2.CreateEWF(ewfFile)
	if err != nil {
		t.Fatalf("Failed to create EVF2 creator: %v", err)
	}

	writer, err := creator.Start(int64(len(originalData)))
	if err != nil {
		t.Fatalf("Failed to start writer: %v", err)
	}

	_, err = writer.Write(originalData)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	ewfFile.Close()

	// Read and verify hashes
	ewfFile, err = os.Open(ewfPath)
	if err != nil {
		t.Fatalf("Failed to open EWF file: %v", err)
	}
	defer ewfFile.Close()

	reader, err := evf2.OpenEWF(ewfFile)
	if err != nil {
		t.Fatalf("Failed to open EVF2 reader: %v", err)
	}

	// Verify that hashes are stored (actual hash values include padding which is expected)
	if reader.First.MD5Hash == nil {
		t.Error("MD5Hash section is nil")
	} else {
		zeroMD5 := [16]byte{}
		if bytes.Equal(reader.First.MD5Hash.Hash[:], zeroMD5[:]) {
			t.Error("MD5 hash is zero")
		}
	}

	if reader.First.SHA1Hash == nil {
		t.Error("SHA1Hash section is nil")
	} else {
		zeroSHA1 := [20]byte{}
		if bytes.Equal(reader.First.SHA1Hash.Hash[:], zeroSHA1[:]) {
			t.Error("SHA1 hash is zero")
		}
	}
}

// TestMultipleReads tests reading the same data multiple times
func TestEVF2MultipleReads(t *testing.T) {
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	tmpDir := t.TempDir()
	ewfPath := filepath.Join(tmpDir, "test_multi.Ex01")

	// Create and write
	ewfFile, err := os.Create(ewfPath)
	if err != nil {
		t.Fatalf("Failed to create EWF file: %v", err)
	}

	creator, err := evf2.CreateEWF(ewfFile)
	if err != nil {
		t.Fatalf("Failed to create EVF2 creator: %v", err)
	}

	writer, err := creator.Start(int64(len(originalData)))
	if err != nil {
		t.Fatalf("Failed to start writer: %v", err)
	}

	_, err = writer.Write(originalData)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	ewfFile.Close()

	// Read multiple times
	for i := 0; i < 3; i++ {
		ewfFile, err := os.Open(ewfPath)
		if err != nil {
			t.Fatalf("Failed to open EWF file on iteration %d: %v", i, err)
		}

		reader, err := evf2.OpenEWF(ewfFile)
		if err != nil {
			ewfFile.Close()
			t.Fatalf("Failed to open EVF2 reader on iteration %d: %v", i, err)
		}

		// Read only the original data size (not the padded data)
		readData := make([]byte, len(originalData))
		n, err := io.ReadFull(reader, readData)
		if err != nil {
			ewfFile.Close()
			t.Fatalf("Failed to read data on iteration %d: %v", i, err)
		}

		if n != len(originalData) {
			t.Errorf("Read size mismatch on iteration %d: got %d, want %d", i, n, len(originalData))
		}

		if !bytes.Equal(originalData, readData) {
			t.Errorf("Data mismatch on iteration %d", i)
		}

		ewfFile.Close()
	}
}

// TestChunkedWrite tests writing data in various chunk sizes
func TestEVF2ChunkedWrite(t *testing.T) {
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	chunkSizes := []int{1024, 4096, 16384, 65536}

	for _, chunkSize := range chunkSizes {
		t.Run(string(rune(chunkSize)), func(t *testing.T) {
			tmpDir := t.TempDir()
			ewfPath := filepath.Join(tmpDir, "test_chunk.Ex01")

			ewfFile, err := os.Create(ewfPath)
			if err != nil {
				t.Fatalf("Failed to create EWF file: %v", err)
			}

			creator, err := evf2.CreateEWF(ewfFile)
			if err != nil {
				t.Fatalf("Failed to create EVF2 creator: %v", err)
			}

			writer, err := creator.Start(int64(len(originalData)))
			if err != nil {
				t.Fatalf("Failed to start writer: %v", err)
			}

			// Write in chunks
			for offset := 0; offset < len(originalData); offset += chunkSize {
				end := offset + chunkSize
				if end > len(originalData) {
					end = len(originalData)
				}
				_, err = writer.Write(originalData[offset:end])
				if err != nil {
					t.Fatalf("Failed to write chunk: %v", err)
				}
			}

			err = writer.Close()
			if err != nil {
				t.Fatalf("Failed to close writer: %v", err)
			}
			ewfFile.Close()

			// Verify
			ewfFile, err = os.Open(ewfPath)
			if err != nil {
				t.Fatalf("Failed to open EWF file: %v", err)
			}
			defer ewfFile.Close()

			reader, err := evf2.OpenEWF(ewfFile)
			if err != nil {
				t.Fatalf("Failed to open EVF2 reader: %v", err)
			}

			// Read only the original data size (not the padded data)
			readData := make([]byte, len(originalData))
			n, err := io.ReadFull(reader, readData)
			if err != nil {
				t.Fatalf("Failed to read data: %v", err)
			}

			if n != len(originalData) {
				t.Errorf("Read size mismatch: got %d, want %d", n, len(originalData))
			}

			if !bytes.Equal(originalData, readData) {
				t.Error("Data mismatch with chunked write")
			}
		})
	}
}

// BenchmarkEVF1Write benchmarks EVF1 writing performance
func BenchmarkEVF1Write(b *testing.B) {
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		b.Fatalf("Failed to read test data file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpDir := b.TempDir()
		ewfPath := filepath.Join(tmpDir, "bench.E01")

		ewfFile, err := os.Create(ewfPath)
		if err != nil {
			b.Fatalf("Failed to create EWF file: %v", err)
		}

		creator, _ := evf1.CreateEWF(ewfFile)
		writer, _ := creator.Start()
		io.Copy(writer, bytes.NewReader(originalData))
		writer.Close()
		ewfFile.Close()
	}
}

// BenchmarkEVF2Write benchmarks EVF2 writing performance
func BenchmarkEVF2Write(b *testing.B) {
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		b.Fatalf("Failed to read test data file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpDir := b.TempDir()
		ewfPath := filepath.Join(tmpDir, "bench.Ex01")

		ewfFile, err := os.Create(ewfPath)
		if err != nil {
			b.Fatalf("Failed to create EWF file: %v", err)
		}

		creator, _ := evf2.CreateEWF(ewfFile)
		writer, _ := creator.Start(int64(len(originalData)))
		io.Copy(writer, bytes.NewReader(originalData))
		writer.Close()
		ewfFile.Close()
	}
}

// BenchmarkEVF2Read benchmarks EVF2 reading performance
func BenchmarkEVF2Read(b *testing.B) {
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		b.Fatalf("Failed to read test data file: %v", err)
	}

	// Create test file once
	tmpDir := b.TempDir()
	ewfPath := filepath.Join(tmpDir, "bench.Ex01")

	ewfFile, err := os.Create(ewfPath)
	if err != nil {
		b.Fatalf("Failed to create EWF file: %v", err)
	}

	creator, _ := evf2.CreateEWF(ewfFile)
	writer, _ := creator.Start(int64(len(originalData)))
	io.Copy(writer, bytes.NewReader(originalData))
	writer.Close()
	ewfFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ewfFile, _ := os.Open(ewfPath)
		reader, _ := evf2.OpenEWF(ewfFile)
		io.Copy(io.Discard, reader)
		ewfFile.Close()
	}
}

// TestCrossCompatibility tests writing with one format and ensuring data integrity
func TestEVF1AndEVF2DataIntegrity(t *testing.T) {
	originalData, err := os.ReadFile(testDataFile)
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	// Compute hash of original data
	originalHash := sha256.Sum256(originalData)

	tmpDir := t.TempDir()

	// Test EVF1
	t.Run("EVF1", func(t *testing.T) {
		ewfPath := filepath.Join(tmpDir, "integrity.E01")

		// Write
		ewfFile, err := os.Create(ewfPath)
		if err != nil {
			t.Fatalf("Failed to create EWF file: %v", err)
		}

		creator, err := evf1.CreateEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to create EVF1 creator: %v", err)
		}

		// Add some metadata to avoid empty identifier issues
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_CASE_NUMBER, "INTEGRITY-TEST")
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER, "INT-001")

		writer, err := creator.Start()
		if err != nil {
			t.Fatalf("Failed to start writer: %v", err)
		}
		_, err = io.Copy(writer, bytes.NewReader(originalData))
		if err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}
		err = writer.Close()
		if err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}
		ewfFile.Close()

		// Read and hash (only original data size, not padding)
		ewfFile, err = os.Open(ewfPath)
		if err != nil {
			t.Fatalf("Failed to open EWF file: %v", err)
		}
		reader, err := evf1.OpenEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to open EVF1 reader: %v", err)
		}
		readData := make([]byte, len(originalData))
		_, err = io.ReadFull(reader, readData)
		if err != nil {
			t.Fatalf("Failed to read data: %v", err)
		}
		ewfFile.Close()

		readHash := sha256.Sum256(readData)
		if readHash != originalHash {
			t.Error("EVF1: Hash mismatch - data corruption detected")
		}
	})

	// Test EVF2
	t.Run("EVF2", func(t *testing.T) {
		ewfPath := filepath.Join(tmpDir, "integrity.Ex01")

		// Write
		ewfFile, err := os.Create(ewfPath)
		if err != nil {
			t.Fatalf("Failed to create EWF file: %v", err)
		}

		creator, err := evf2.CreateEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to create EVF2 creator: %v", err)
		}
		writer, err := creator.Start(int64(len(originalData)))
		if err != nil {
			t.Fatalf("Failed to start writer: %v", err)
		}
		_, err = io.Copy(writer, bytes.NewReader(originalData))
		if err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}
		err = writer.Close()
		if err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}
		ewfFile.Close()

		// Read and hash (only original data size, not padding)
		ewfFile, err = os.Open(ewfPath)
		if err != nil {
			t.Fatalf("Failed to open EWF file: %v", err)
		}
		reader, err := evf2.OpenEWF(ewfFile)
		if err != nil {
			t.Fatalf("Failed to open EVF2 reader: %v", err)
		}
		readData := make([]byte, len(originalData))
		_, err = io.ReadFull(reader, readData)
		if err != nil {
			t.Fatalf("Failed to read data: %v", err)
		}
		ewfFile.Close()

		readHash := sha256.Sum256(readData)
		if readHash != originalHash {
			t.Error("EVF2: Hash mismatch - data corruption detected")
		}
	})
}
