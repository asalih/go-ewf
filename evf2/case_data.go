package evf2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/asalih/go-ewf/shared"
)

type EWFCaseDataInformationKey string

const (
	EWF_CASE_DATA_NAME                 EWFCaseDataInformationKey = "nm"
	EWF_CASE_DATA_CASE_NUMBER          EWFCaseDataInformationKey = "cn"
	EWF_CASE_DATA_EVIDENCE_NUMBER      EWFCaseDataInformationKey = "en"
	EWF_CASE_DATA_EXAMINER_NAME        EWFCaseDataInformationKey = "ex"
	EWF_CASE_DATA_NOTES                EWFCaseDataInformationKey = "nt"
	EWF_CASE_DATA_APPLICATION_VERSION  EWFCaseDataInformationKey = "av"
	EWF_CASE_DATA_OS                   EWFCaseDataInformationKey = "os"
	EWF_CASE_DATA_TARGET_TIME          EWFCaseDataInformationKey = "tt"
	EWF_CASE_DATA_ACTUAL_TIME          EWFCaseDataInformationKey = "at"
	EWF_CASE_DATA_NUMBER_OF_CHUNKS     EWFCaseDataInformationKey = "tb"
	EWF_CASE_DATA_COMPRESSION_METHOD   EWFCaseDataInformationKey = "cp"
	EWF_CASE_DATA_NUMBER_OF_SECTORS_PC EWFCaseDataInformationKey = "sb"
	EWF_CASE_DATA_ERROR_GRANULARITY    EWFCaseDataInformationKey = "gr"
	EWF_CASE_DATA_WRITE_BLOCKER_TYPE   EWFCaseDataInformationKey = "wb"
)

var CaseDataIdentifiers = map[EWFCaseDataInformationKey]string{
	EWF_CASE_DATA_NAME:                 "Name",
	EWF_CASE_DATA_CASE_NUMBER:          "Case Number",
	EWF_CASE_DATA_EVIDENCE_NUMBER:      "Evidence Number",
	EWF_CASE_DATA_EXAMINER_NAME:        "Examiner name",
	EWF_CASE_DATA_NOTES:                "Notes",
	EWF_CASE_DATA_APPLICATION_VERSION:  "Application Version",
	EWF_CASE_DATA_OS:                   "Operatin system",
	EWF_CASE_DATA_TARGET_TIME:          "Target Time",
	EWF_CASE_DATA_ACTUAL_TIME:          "Actual Time",
	EWF_CASE_DATA_NUMBER_OF_CHUNKS:     "Number of chunks",
	EWF_CASE_DATA_COMPRESSION_METHOD:   "Compression method",
	EWF_CASE_DATA_NUMBER_OF_SECTORS_PC: "Number of sectors per chunk",
	EWF_CASE_DATA_ERROR_GRANULARITY:    "Error granularity",
	EWF_CASE_DATA_WRITE_BLOCKER_TYPE:   "Write-blocker type",
}

type EWFCaseDataSection struct {
	NumberOfObjects string
	ObjectName      string
	KeyValue        map[string]string

	// Cached parsed values for hot path performance
	cachedSectorCount int
	cachedChunkCount  int
}

func (ewfHeader *EWFCaseDataSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, decompressorFunc shared.Decompressor) error {
	fh.Seek(section.DataOffset, io.SeekStart)
	rd := make([]byte, section.Size)
	if _, err := fh.Read(rd); err != nil {
		return err
	}

	data, err := decompressorFunc(rd)
	if err != nil {
		return err
	}

	// Starts with a BOM
	if data[0] == 255 || data[1] == 254 {
		data = []byte(shared.UTF16ToUTF8(data))
	}

	var identifiers []string
	var values []string

	for lineNum, line := range bytes.Split(data, newLineDelim) {
		for _, attr := range bytes.Split(line, fieldDelim) {
			strAttr := string(bytes.TrimSuffix(attr, []byte{'\r'}))
			if lineNum == 0 {
				ewfHeader.NumberOfObjects = string(strAttr[0])
			} else if lineNum == 1 {
				ewfHeader.ObjectName = strAttr
			} else if lineNum == 2 {
				_, ok := CaseDataIdentifiers[EWFCaseDataInformationKey(strAttr)]
				if !ok {
					return fmt.Errorf("media identifier is unknown: %s", strAttr)
				}
				identifiers = append(identifiers, strAttr)
			} else if lineNum == 3 {
				values = append(values, strAttr)
			}
		}
	}
	ewfHeader.KeyValue = shared.ToMap(identifiers, values)

	// Pre-parse and cache values for hot path performance
	if sb, ok := ewfHeader.KeyValue[string(EWF_CASE_DATA_NUMBER_OF_SECTORS_PC)]; ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(sb)); err == nil {
			ewfHeader.cachedSectorCount = parsed
		}
	}
	if tb, ok := ewfHeader.KeyValue[string(EWF_CASE_DATA_NUMBER_OF_CHUNKS)]; ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(tb)); err == nil {
			ewfHeader.cachedChunkCount = parsed
		}
	}

	return nil
}

// Encode writes data and its description to the target writer. Returns  data write count, descriptor write count and err
func (ewfHeader *EWFCaseDataSection) Encode(ewf io.Writer, previousDescriptorPosition int64) (dataN int, descN int, err error) {
	buf := bytes.NewBuffer(nil)

	buf.WriteString(ewfHeader.NumberOfObjects)
	buf.Write(newLineDelim)
	buf.WriteString(ewfHeader.ObjectName)
	buf.Write(newLineDelim)

	mk := make([]string, 0, len(ewfHeader.KeyValue))
	mv := make([]string, 0, len(ewfHeader.KeyValue))
	for k, v := range ewfHeader.KeyValue {
		mk = append(mk, k)
		mv = append(mv, v)
	}
	buf.WriteString(strings.Join(mk, string(fieldDelim)))
	buf.Write(newLineDelim)
	buf.WriteString(strings.Join(mv, string(fieldDelim)))
	buf.Write(newLineDelim)
	buf.Write(newLineDelim)

	utf16Data := shared.UTF8ToUTF16(buf.Bytes())
	comp, err := shared.NewZlibCompressor()
	if err != nil {
		return 0, 0, err
	}
	zlHeader, err := comp.Compress(utf16Data)
	if err != nil {
		return 0, 0, err
	}

	zlHeader, paddingSize := alignTo16Bytes(zlHeader[:])

	dataN, err = ewf.Write(zlHeader)
	if err != nil {
		return 0, 0, err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_CASE_DATA)

	desc.DataSize = uint64(len(zlHeader))
	desc.PreviousOffset = uint64(previousDescriptorPosition)
	desc.PaddingSize = uint32(paddingSize)

	// header section and its data appears twice subsequently
	// after first write, we arrange the "Next" field then write
	descN, chc, err := shared.WriteWithSum(ewf, desc)
	if err != nil {
		return 0, 0, err
	}
	desc.Checksum = chc
	return dataN, descN, err
}

func (c *EWFCaseDataSection) GetSectorCount() (int, error) {
	if c.cachedSectorCount > 0 {
		return c.cachedSectorCount, nil
	}
	sb, ok := c.KeyValue[string(EWF_CASE_DATA_NUMBER_OF_SECTORS_PC)]
	if !ok {
		return 0, errors.New("case data has no sector count")
	}
	return strconv.Atoi(strings.TrimSpace(sb))
}

func (c *EWFCaseDataSection) GetChunkCount() (int, error) {
	if c.cachedChunkCount > 0 {
		return c.cachedChunkCount, nil
	}
	tb, ok := c.KeyValue[string(EWF_CASE_DATA_NUMBER_OF_CHUNKS)]
	if !ok {
		return 0, errors.New("case data has no chunk count")
	}
	return strconv.Atoi(strings.TrimSpace(tb))
}
