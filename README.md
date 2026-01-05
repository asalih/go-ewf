# go-ewf

Golang EWF Reader/Writer implementation

[![Go Report Card](https://goreportcard.com/badge/github.com/asalih/go-ewf)](https://goreportcard.com/report/github.com/asalih/go-ewf)
[![Tests](https://github.com/asalih/go-ewf/actions/workflows/test.yml/badge.svg)](https://github.com/asalih/go-ewf/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/asalih/go-ewf.svg)](https://pkg.go.dev/github.com/asalih/go-ewf)
[![License](https://img.shields.io/github/license/asalih/go-ewf)](LICENSE)

## Features

| Function | EWF (E01) | EWF2 (Ex01) |
| --- | --- | --- |
| Reader | ✅ | ✅ |
| Writer | ✅ | ✅ |
| Metadata | ✅ | ✅ |
| Compression | ✅ (zlib) | ✅ (zlib, bzip2) |
| Hash Verification | ✅ (MD5, SHA1) | ✅ (MD5, SHA1) |
| Random Access | ✅ | ✅ |

## Installation

### As a Library

```bash
go get github.com/asalih/go-ewf
```

### As a CLI Tool

```bash
# Clone the repository
git clone https://github.com/asalih/go-ewf.git
cd go-ewf

# Build the CLI tool
go build -o ewf-tool ./cmd/main.go

# Or install globally
go install ./cmd/...
```

## CLI Tool

The project includes a powerful command-line tool for creating, dumping, and inspecting EWF images.

```bash
# Create an EWF image from raw data
ewf-tool create -source disk.dd -target disk.Ex01 -format evf2 \
  -case-number "CASE-001" -examiner "John Doe" -verbose

# Extract data from EWF image
ewf-tool dump -source evidence.E01 -target recovered.dd -verbose

# Display image information and metadata
ewf-tool info -source evidence.Ex01

# Extract specific range
ewf-tool dump -source disk.Ex01 -target partition.raw \
  -offset 1048576 -length 104857600
```

See [CLI_USAGE.md](CLI_USAGE.md) for complete CLI documentation and examples.

## Library Usage

### Reading EWF Files

```go
package main

import (
    "fmt"
    "io"
    "os"
    
    "github.com/asalih/go-ewf/evf1"
    "github.com/asalih/go-ewf/evf2"
)

func main() {
    // Open an E01 file
    file, _ := os.Open("image.E01")
    defer file.Close()
    
    // For EVF1 (E01)
    reader, _ := evf1.OpenEWF(file)
    
    // Or for EVF2 (Ex01)
    // reader, _ := evf2.OpenEWF(file)
    
    // Get metadata
    metadata := reader.Metadata()
    fmt.Printf("Metadata: %+v\n", metadata)
    
    // Get size
    fmt.Printf("Image size: %d bytes\n", reader.Size())
    
    // Read data
    buf := make([]byte, 4096)
    n, _ := reader.Read(buf)
    fmt.Printf("Read %d bytes\n", n)
    
    // Random access
    reader.ReadAt(buf, 1024*1024) // Read at 1MB offset
}
```

### Writing EWF Files

```go
package main

import (
    "io"
    "os"
    
    "github.com/asalih/go-ewf/evf1"
    "github.com/asalih/go-ewf/evf2"
)

func main() {
    // Create output file
    outFile, _ := os.Create("output.Ex01")
    defer outFile.Close()
    
    // Create EVF2 writer
    creator, _ := evf2.CreateEWF(outFile)
    
    // Add metadata
    creator.AddCaseData(evf2.EWF_CASE_DATA_CASE_NUMBER, "CASE-001")
    creator.AddCaseData(evf2.EWF_CASE_DATA_EXAMINER_NAME, "John Doe")
    creator.AddDeviceInformation(evf2.EWF_DEVICE_INFO_DRIVE_MODEL, "Virtual Drive")
    
    // Start writing (provide total size for EVF2)
    sourceFile, _ := os.Open("source.dd")
    defer sourceFile.Close()
    
    stat, _ := sourceFile.Stat()
    writer, _ := creator.Start(stat.Size())
    
    // Write data
    io.Copy(writer, sourceFile)
    
    // Close (computes and writes hashes)
    writer.Close()
}
```

## Testing

The project includes comprehensive integration tests using real-world test data (8.6 MB).

### Run Tests

```bash
# Run all tests
go test -v

# Run with coverage
go test -v -cover

# Run benchmarks
go test -bench=. -benchtime=3s
```

### Test Coverage

- ✅ Complete write/read cycles for both EVF1 and EVF2
- ✅ Metadata preservation and retrieval
- ✅ Random access reads at various offsets
- ✅ Seek operations (SeekStart, SeekCurrent, SeekEnd)
- ✅ Hash computation and verification (MD5, SHA1)
- ✅ Multiple reads from same file
- ✅ Various write chunk sizes
- ✅ Data integrity verification

See [TESTING.md](TESTING.md) for detailed test documentation.

## Performance

Benchmarks on Apple M3 Pro with 8.6 MB test file:

| Operation | Performance |
| --- | --- |
| EVF1 Write | ~32 MB/s |
| EVF2 Write | ~31 MB/s |
| EVF2 Read | ~76 MB/s |

## CI/CD

The project uses GitHub Actions for continuous integration:

- **Tests**: Run on Linux, macOS, and Windows with Go 1.21, 1.22, and 1.23
- **Benchmarks**: Performance regression testing
- **Linting**: Code quality checks with golangci-lint
- **Build**: Verify builds on all platforms

## License

See [LICENSE](LICENSE) file for details.