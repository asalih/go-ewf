package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asalih/go-ewf/evf1"
	"github.com/asalih/go-ewf/evf2"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "dump":
		dumpCommand()
	case "create":
		createCommand()
	case "info":
		infoCommand()
	case "version":
		fmt.Printf("go-ewf version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`go-ewf - EWF/E01 Image Tool

Usage:
  go-ewf <command> [options]

Commands:
  dump      Extract data from an EWF image to raw format
  create    Create an EWF image from raw data
  info      Display information about an EWF image
  version   Show version information
  help      Show this help message

Run 'go-ewf <command> -h' for more information on a command.`)
}

// dumpCommand extracts data from an EWF image
func dumpCommand() {
	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	source := fs.String("source", "", "Source EWF image file (required)")
	target := fs.String("target", "", "Target output file (required)")
	offset := fs.Int64("offset", 0, "Start offset in bytes")
	length := fs.Int64("length", -1, "Number of bytes to extract (-1 for all)")
	bufferSize := fs.Int("buffer", 1024*1024, "Buffer size in bytes (default: 1MB)")
	verbose := fs.Bool("verbose", false, "Verbose output")

	fs.Parse(os.Args[2:])

	if *source == "" || *target == "" {
		fmt.Fprintf(os.Stderr, "Error: -source and -target are required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	if err := dumpImage(*source, *target, *offset, *length, *bufferSize, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// createCommand creates an EWF image from raw data
func createCommand() {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	source := fs.String("source", "", "Source raw data file (required)")
	target := fs.String("target", "", "Target EWF image file (required)")
	format := fs.String("format", "evf2", "EWF format: evf1 (E01) or evf2 (Ex01)")

	// Metadata flags
	caseNumber := fs.String("case-number", "", "Case number")
	evidenceNumber := fs.String("evidence-number", "", "Evidence number")
	examiner := fs.String("examiner", "", "Examiner name")
	notes := fs.String("notes", "", "Notes")
	description := fs.String("description", "", "Description")

	// Device info (EVF2 only)
	driveModel := fs.String("drive-model", "", "Drive model (EVF2 only)")
	serialNumber := fs.String("serial-number", "", "Serial number (EVF2 only)")

	bufferSize := fs.Int("buffer", 1024*1024, "Buffer size in bytes (default: 1MB)")
	verbose := fs.Bool("verbose", false, "Verbose output")

	fs.Parse(os.Args[2:])

	if *source == "" || *target == "" {
		fmt.Fprintf(os.Stderr, "Error: -source and -target are required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	metadata := Metadata{
		CaseNumber:     *caseNumber,
		EvidenceNumber: *evidenceNumber,
		Examiner:       *examiner,
		Notes:          *notes,
		Description:    *description,
		DriveModel:     *driveModel,
		SerialNumber:   *serialNumber,
	}

	if err := createImage(*source, *target, *format, metadata, *bufferSize, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// infoCommand displays information about an EWF image
func infoCommand() {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	source := fs.String("source", "", "Source EWF image file (required)")

	fs.Parse(os.Args[2:])

	if *source == "" {
		fmt.Fprintf(os.Stderr, "Error: -source is required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	if err := showImageInfo(*source); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type Metadata struct {
	CaseNumber     string
	EvidenceNumber string
	Examiner       string
	Notes          string
	Description    string
	DriveModel     string
	SerialNumber   string
}

func dumpImage(source, target string, offset, length int64, bufferSize int, verbose bool) error {
	if verbose {
		fmt.Printf("Opening EWF image: %s\n", source)
	}

	// Open source file
	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	// Try to detect format and open
	var reader io.ReadSeeker
	var size int64

	// Try EVF2 first
	ewf2, err := evf2.OpenEWF(sourceFile)
	if err == nil {
		reader = ewf2
		size = ewf2.Size()
		if verbose {
			fmt.Printf("Detected format: EVF2 (Ex01)\n")
			fmt.Printf("Image size: %d bytes (%.2f GB)\n", size, float64(size)/(1024*1024*1024))
		}
	} else {
		// Reset and try EVF1
		if _, err := sourceFile.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to start: %w", err)
		}
		ewf1, err := evf1.OpenEWF(sourceFile)
		if err != nil {
			return fmt.Errorf("failed to open as EVF1 or EVF2: %w", err)
		}
		reader = ewf1
		size = ewf1.Size()
		if verbose {
			fmt.Printf("Detected format: EVF1 (E01)\n")
			fmt.Printf("Image size: %d bytes (%.2f GB)\n", size, float64(size)/(1024*1024*1024))
		}
	}

	// Validate offset and length
	if offset >= size {
		return fmt.Errorf("offset %d is beyond image size %d", offset, size)
	}

	if length == -1 {
		length = size - offset
	}

	if offset+length > size {
		length = size - offset
	}

	if verbose {
		fmt.Printf("Extracting %d bytes from offset %d\n", length, offset)
	}

	// Create target file
	targetFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer func() {
		_ = targetFile.Close()
	}()

	// Seek to offset
	if _, err := reader.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to offset: %w", err)
	}

	// Copy data with progress
	buffer := make([]byte, bufferSize)
	var copied int64
	startTime := time.Now()
	lastUpdate := time.Now()

	for copied < length {
		toRead := int64(bufferSize)
		if length-copied < toRead {
			toRead = length - copied
		}

		n, err := reader.Read(buffer[:toRead])
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read from source: %w", err)
		}

		if n == 0 {
			break
		}

		if _, err := targetFile.Write(buffer[:n]); err != nil {
			return fmt.Errorf("failed to write to target: %w", err)
		}

		copied += int64(n)

		// Show progress every second
		if verbose && time.Since(lastUpdate) >= time.Second {
			elapsed := time.Since(startTime)
			rate := float64(copied) / elapsed.Seconds() / (1024 * 1024)
			progress := float64(copied) / float64(length) * 100
			fmt.Printf("\rProgress: %.2f%% (%d/%d bytes) - %.2f MB/s", progress, copied, length, rate)
			lastUpdate = time.Now()
		}
	}

	if verbose {
		elapsed := time.Since(startTime)
		rate := float64(copied) / elapsed.Seconds() / (1024 * 1024)
		fmt.Printf("\rCompleted: 100.00%% (%d bytes) - %.2f MB/s in %s\n", copied, rate, elapsed.Round(time.Second))
	}

	return nil
}

func createImage(source, target, format string, metadata Metadata, bufferSize int, verbose bool) error {
	format = strings.ToLower(format)
	if format != "evf1" && format != "evf2" {
		return fmt.Errorf("invalid format: %s (must be evf1 or evf2)", format)
	}

	if verbose {
		fmt.Printf("Creating %s image from: %s\n", strings.ToUpper(format), source)
	}

	// Open source file
	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	sourceSize := sourceInfo.Size()
	if verbose {
		fmt.Printf("Source size: %d bytes (%.2f GB)\n", sourceSize, float64(sourceSize)/(1024*1024*1024))
	}

	// Ensure target has correct extension
	if format == "evf1" && !strings.HasSuffix(strings.ToLower(target), ".e01") {
		target = target + ".E01"
	} else if format == "evf2" && !strings.HasSuffix(strings.ToLower(target), ".ex01") {
		target = target + ".Ex01"
	}

	// Create target file
	targetFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer func() {
		_ = targetFile.Close()
	}()

	if verbose {
		fmt.Printf("Output file: %s\n", target)
	}

	startTime := time.Now()
	var written int64

	if format == "evf1" {
		written, err = createEVF1Image(sourceFile, targetFile, sourceSize, metadata, bufferSize, verbose)
	} else {
		written, err = createEVF2Image(sourceFile, targetFile, sourceSize, metadata, bufferSize, verbose)
	}

	if err != nil {
		return err
	}

	if verbose {
		elapsed := time.Since(startTime)
		rate := float64(written) / elapsed.Seconds() / (1024 * 1024)
		fmt.Printf("\nCompleted: %d bytes written in %s (%.2f MB/s)\n", written, elapsed.Round(time.Second), rate)
	}

	return nil
}

func createEVF1Image(source io.Reader, target io.WriteSeeker, size int64, metadata Metadata, bufferSize int, verbose bool) (int64, error) {
	creator, err := evf1.CreateEWF(target)
	if err != nil {
		return 0, fmt.Errorf("failed to create EVF1 writer: %w", err)
	}

	// Add metadata
	if metadata.CaseNumber != "" {
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_CASE_NUMBER, metadata.CaseNumber)
	}
	if metadata.EvidenceNumber != "" {
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER, metadata.EvidenceNumber)
	}
	if metadata.Examiner != "" {
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EXAMINER_NAME, metadata.Examiner)
	}
	if metadata.Notes != "" {
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_NOTES, metadata.Notes)
	}
	if metadata.Description != "" {
		creator.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_DESCRIPTION, metadata.Description)
	}

	writer, err := creator.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to start writer: %w", err)
	}

	// Copy data with progress
	buffer := make([]byte, bufferSize)
	var written int64
	lastUpdate := time.Now()

	for {
		n, err := source.Read(buffer)
		if err != nil && err != io.EOF {
			return written, fmt.Errorf("failed to read from source: %w", err)
		}

		if n == 0 {
			break
		}

		if _, err := writer.Write(buffer[:n]); err != nil {
			return written, fmt.Errorf("failed to write to EWF: %w", err)
		}

		written += int64(n)

		// Show progress every second
		if verbose && time.Since(lastUpdate) >= time.Second {
			progress := float64(written) / float64(size) * 100
			fmt.Printf("\rProgress: %.2f%% (%d/%d bytes)", progress, written, size)
			lastUpdate = time.Now()
		}
	}

	if err := writer.Close(); err != nil {
		return written, fmt.Errorf("failed to close writer: %w", err)
	}

	return written, nil
}

func createEVF2Image(source io.Reader, target io.Writer, size int64, metadata Metadata, bufferSize int, verbose bool) (int64, error) {
	creator, err := evf2.CreateEWF(target)
	if err != nil {
		return 0, fmt.Errorf("failed to create EVF2 writer: %w", err)
	}

	// Add case data metadata
	if metadata.CaseNumber != "" {
		creator.AddCaseData(evf2.EWF_CASE_DATA_CASE_NUMBER, metadata.CaseNumber)
	}
	if metadata.EvidenceNumber != "" {
		creator.AddCaseData(evf2.EWF_CASE_DATA_EVIDENCE_NUMBER, metadata.EvidenceNumber)
	}
	if metadata.Examiner != "" {
		creator.AddCaseData(evf2.EWF_CASE_DATA_EXAMINER_NAME, metadata.Examiner)
	}
	if metadata.Notes != "" {
		creator.AddCaseData(evf2.EWF_CASE_DATA_NOTES, metadata.Notes)
	}
	if metadata.Description != "" {
		creator.AddCaseData(evf2.EWF_CASE_DATA_NAME, metadata.Description)
	}

	// Add device information metadata
	if metadata.DriveModel != "" {
		creator.AddDeviceInformation(evf2.EWF_DEVICE_INFO_DRIVE_MODEL, metadata.DriveModel)
	}
	if metadata.SerialNumber != "" {
		creator.AddDeviceInformation(evf2.EWF_DEVICE_INFO_SERIAL_NUMBER, metadata.SerialNumber)
	}

	writer, err := creator.Start(size)
	if err != nil {
		return 0, fmt.Errorf("failed to start writer: %w", err)
	}

	// Copy data with progress
	buffer := make([]byte, bufferSize)
	var written int64
	lastUpdate := time.Now()

	for {
		n, err := source.Read(buffer)
		if err != nil && err != io.EOF {
			return written, fmt.Errorf("failed to read from source: %w", err)
		}

		if n == 0 {
			break
		}

		if _, err := writer.Write(buffer[:n]); err != nil {
			return written, fmt.Errorf("failed to write to EWF: %w", err)
		}

		written += int64(n)

		// Show progress every second
		if verbose && time.Since(lastUpdate) >= time.Second {
			progress := float64(written) / float64(size) * 100
			fmt.Printf("\rProgress: %.2f%% (%d/%d bytes)", progress, written, size)
			lastUpdate = time.Now()
		}
	}

	if err := writer.Close(); err != nil {
		return written, fmt.Errorf("failed to close writer: %w", err)
	}

	return written, nil
}

func showImageInfo(source string) error {
	// Open source file
	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	// Try EVF2 first
	ewf2, err := evf2.OpenEWF(sourceFile)
	if err == nil {
		return showEVF2Info(source, ewf2)
	}

	// Try EVF1
	if _, err := sourceFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start: %w", err)
	}
	ewf1, err := evf1.OpenEWF(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to open as EVF1 or EVF2: %w", err)
	}

	return showEVF1Info(source, ewf1)
}

func showEVF1Info(source string, reader *evf1.EWFReader) error {
	fmt.Printf("EWF Image Information\n")
	fmt.Printf("=====================\n\n")
	fmt.Printf("File: %s\n", filepath.Base(source))
	fmt.Printf("Format: EVF1 (E01)\n")
	fmt.Printf("Size: %d bytes (%.2f GB)\n", reader.Size(), float64(reader.Size())/(1024*1024*1024))
	fmt.Printf("Chunk Size: %d bytes\n", reader.ChunkSize)
	fmt.Printf("\nMetadata:\n")

	metadata := reader.Metadata()
	for key, value := range metadata {
		if strValue, ok := value.(string); ok && strValue != "" {
			fmt.Printf("  %s: %s\n", key, strValue)
		}
	}

	return nil
}

func showEVF2Info(source string, reader *evf2.EWFReader) error {
	fmt.Printf("EWF Image Information\n")
	fmt.Printf("=====================\n\n")
	fmt.Printf("File: %s\n", filepath.Base(source))
	fmt.Printf("Format: EVF2 (Ex01)\n")
	fmt.Printf("Size: %d bytes (%.2f GB)\n", reader.Size(), float64(reader.Size())/(1024*1024*1024))
	fmt.Printf("Chunk Size: %d bytes\n", reader.ChunkSize)
	fmt.Printf("\nMetadata:\n")

	metadata := reader.Metadata()
	for section, data := range metadata {
		if sectionData, ok := data.(map[string]string); ok {
			fmt.Printf("\n  %s:\n", section)
			for key, value := range sectionData {
				if value != "" {
					fmt.Printf("    %s: %s\n", key, value)
				}
			}
		}
	}

	return nil
}
