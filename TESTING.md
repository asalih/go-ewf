# Integration Tests for go-ewf

This document describes the integration tests for the EWF (Expert Witness Format) reader and writer implementations.

## Test Data

All tests use the real-world test data file `testdata/winevt-rc.db` (8.6 MB) to ensure comprehensive testing with actual data.

## Test Coverage

### 1. TestEVF1WriteRead

Tests the complete write and read cycle for EVF1 (E01) format.

**Subtests:**
- **Write**: Creates an EVF1 file with metadata and writes the test data
  - Verifies metadata can be added (case number, examiner name, evidence number, notes, description)
  - Writes data using `io.Copy` in a streaming fashion
  - Ensures proper file closure
  
- **Read**: Opens and reads the created EVF1 file
  - Verifies the file size (accounting for chunk padding)
  - Validates metadata is correctly stored and retrieved
  - Reads the data and compares with original
  
- **RandomAccess**: Tests random access reads at various offsets
  - Tests reading at offset 0 (start)
  - Tests reading at offset 512 (second sector)
  - Tests reading at offset 1024 (1KB)
  - Tests reading at middle of file
  - Tests reading near end of file

### 2. TestEVF2WriteRead

Tests the complete write and read cycle for EVF2 (Ex01) format.

**Subtests:**
- **Write**: Creates an EVF2 file with metadata
  - Adds case data metadata (case number, examiner name, evidence number, notes, name)
  - Adds device information metadata (drive model, serial number)
  - Writes data with known total size
  
- **Read**: Opens and reads the created EVF2 file
  - Verifies file size (accounting for chunk padding)
  - Validates case data and device information metadata
  - Reads and verifies data integrity
  
- **RandomAccess**: Tests random access reads at various offsets (similar to EVF1)
  
- **Seek**: Tests seek functionality
  - SeekStart: Seeks to absolute position
  - SeekCurrent: Seeks relative to current position
  - SeekEnd: Seeks relative to end of file

### 3. TestEVF1HashVerification

Tests that MD5 and SHA1 hashes are correctly computed and stored for EVF1 format.

- Creates an EVF1 file with test data
- Verifies that the Digest section contains non-zero MD5 and SHA1 hashes
- Note: Hash values include padding as per EWF format specification

### 4. TestEVF2HashVerification

Tests that MD5 and SHA1 hashes are correctly computed and stored for EVF2 format.

- Creates an EVF2 file with test data
- Verifies MD5Hash and SHA1Hash sections exist and contain non-zero values
- Note: Hash values include padding as per EWF format specification

### 5. TestEVF2MultipleReads

Tests reading the same EVF2 file multiple times to ensure consistency.

- Creates a single EVF2 file
- Opens and reads it 3 times
- Verifies data integrity on each read
- Ensures proper resource cleanup

### 6. TestEVF2ChunkedWrite

Tests writing data in various chunk sizes to ensure proper handling.

**Chunk sizes tested:**
- 1024 bytes
- 4096 bytes
- 16384 bytes
- 65536 bytes

For each chunk size:
- Writes the entire test data in chunks of that size
- Reads back the data
- Verifies data integrity

### 7. TestEVF1AndEVF2DataIntegrity

Tests overall data integrity using SHA256 hashing for both formats.

**Subtests:**
- **EVF1**: 
  - Writes test data to EVF1 format
  - Reads back and computes SHA256 hash
  - Compares with original data hash
  
- **EVF2**:
  - Writes test data to EVF2 format
  - Reads back and computes SHA256 hash
  - Compares with original data hash

## Benchmarks

### BenchmarkEVF1Write
Measures the performance of writing 8.6 MB of data to EVF1 format.

### BenchmarkEVF2Write
Measures the performance of writing 8.6 MB of data to EVF2 format.

### BenchmarkEVF2Read
Measures the performance of reading an 8.6 MB EVF2 file.

## Running the Tests

### Run all tests:
```bash
go test -v -timeout 5m
```

### Run specific test:
```bash
go test -v -run TestEVF2WriteRead
```

### Run benchmarks:
```bash
go test -bench=. -benchtime=3s
```

### Run benchmarks with memory profiling:
```bash
go test -bench=. -benchmem
```

## Important Notes

### Chunk Padding

Both EVF1 and EVF2 formats work with fixed-size chunks (32768 bytes by default). When writing data that doesn't align perfectly with chunk boundaries, the last chunk is padded with zeros. This means:

- The reported size from `reader.Size()` may be larger than the original data size
- When reading, you should read exactly the original data size, not the padded size
- Hash values stored in the EWF file include the padding (this is by design)

### Metadata Requirements

For EVF1 format:
- At least some metadata fields must be populated to avoid empty identifier issues
- Common fields: case number, examiner name, evidence number, notes, description

For EVF2 format:
- Case data and device information are separate metadata sections
- Total size must be provided upfront when starting the writer

### Test Data

The test file `testdata/winevt-rc.db` is a real Windows Event Log resource database file, providing a good mix of:
- Binary data
- Text data
- Various compression patterns
- Size that exercises multiple chunks (274 chunks at 32KB each)

## Test Results

All tests pass successfully, demonstrating:
- ✅ Correct data writing and reading for both EVF1 and EVF2
- ✅ Metadata preservation
- ✅ Random access functionality
- ✅ Seek operations
- ✅ Hash computation and storage
- ✅ Data integrity across multiple reads
- ✅ Proper handling of various write chunk sizes
- ✅ Overall data integrity verification

Performance (Apple M3 Pro):
- EVF1 Write: ~267 ms for 8.6 MB (~32 MB/s)
- EVF2 Write: ~277 ms for 8.6 MB (~31 MB/s)
- EVF2 Read: ~113 ms for 8.6 MB (~76 MB/s)

