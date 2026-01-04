package evf1

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/asalih/go-ewf/shared"
)

type EWFMediaInfo string

const (
	EWF_HEADER_VALUES_INDEX_DESCRIPTION              EWFMediaInfo = "a"
	EWF_HEADER_VALUES_INDEX_CASE_NUMBER              EWFMediaInfo = "c"
	EWF_HEADER_VALUES_INDEX_EXAMINER_NAME            EWFMediaInfo = "e"
	EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER          EWFMediaInfo = "n"
	EWF_HEADER_VALUES_INDEX_NOTES                    EWFMediaInfo = "t"
	EWF_HEADER_VALUES_INDEX_ACQUIRY_SOFTWARE_VERSION EWFMediaInfo = "av"
	EWF_HEADER_VALUES_INDEX_ACQUIRY_OPERATING_SYSTEM EWFMediaInfo = "ov"
	EWF_HEADER_VALUES_INDEX_ACQUIRY_DATE             EWFMediaInfo = "m"
	EWF_HEADER_VALUES_INDEX_SYSTEM_DATE              EWFMediaInfo = "u"
	EWF_HEADER_VALUES_INDEX_PASSWORD                 EWFMediaInfo = "p"
	EWF_HEADER_VALUES_INDEX_PROCESS_IDENTIFIER       EWFMediaInfo = "pid"
	EWF_HEADER_VALUES_INDEX_UNKNOWN_DC               EWFMediaInfo = "dc"
	EWF_HEADER_VALUES_INDEX_EXTENTS                  EWFMediaInfo = "ext"
	EWF_HEADER_VALUES_INDEX_COMPRESSION_TYPE         EWFMediaInfo = "r"
	EWF_HEADER_VALUES_INDEX_MODEL                    EWFMediaInfo = "md"
	EWF_HEADER_VALUES_INDEX_SERIAL_NUMBER            EWFMediaInfo = "sn"
	EWF_HEADER_VALUES_INDEX_DEVICE_LABEL             EWFMediaInfo = "l"
)

const (
	EWF_HEADER_VALUES_INDEX_COMPRESSION_BEST    = "b"
	EWF_HEADER_VALUES_INDEX_COMPRESSION_FASTEST = "f"
	EWF_HEADER_VALUES_INDEX_COMPRESSION_NO      = "n"
)

var CompressionLevels = map[string]string{
	EWF_HEADER_VALUES_INDEX_COMPRESSION_BEST:    "Best",
	EWF_HEADER_VALUES_INDEX_COMPRESSION_FASTEST: "Fastest",
	EWF_HEADER_VALUES_INDEX_COMPRESSION_NO:      "No compression",
}

var AcquiredMediaIdentifiers = map[EWFMediaInfo]string{
	EWF_HEADER_VALUES_INDEX_DESCRIPTION:              "Description",
	EWF_HEADER_VALUES_INDEX_CASE_NUMBER:              "Case Number",
	EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER:          "Evidence Number",
	EWF_HEADER_VALUES_INDEX_EXAMINER_NAME:            "Examiner Name",
	EWF_HEADER_VALUES_INDEX_NOTES:                    "Notes",
	EWF_HEADER_VALUES_INDEX_MODEL:                    "Media model",
	EWF_HEADER_VALUES_INDEX_SERIAL_NUMBER:            "Serial number",
	EWF_HEADER_VALUES_INDEX_DEVICE_LABEL:             "Device label",
	EWF_HEADER_VALUES_INDEX_ACQUIRY_SOFTWARE_VERSION: "Version",
	EWF_HEADER_VALUES_INDEX_ACQUIRY_OPERATING_SYSTEM: "Platform",
	EWF_HEADER_VALUES_INDEX_ACQUIRY_DATE:             "Acquired Date",
	EWF_HEADER_VALUES_INDEX_SYSTEM_DATE:              "System Date",
	EWF_HEADER_VALUES_INDEX_PASSWORD:                 "Password Hash",
	EWF_HEADER_VALUES_INDEX_PROCESS_IDENTIFIER:       "Process Identifiers",
	EWF_HEADER_VALUES_INDEX_UNKNOWN_DC:               "Unknown",
	EWF_HEADER_VALUES_INDEX_EXTENTS:                  "Extents",
	EWF_HEADER_VALUES_INDEX_COMPRESSION_TYPE:         "Compression level",
}

type EWFHeaderSection struct {
	NofCategories string
	CategoryName  string
	MediaInfo     map[string]string
}

func (ewfHeader *EWFHeaderSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor) error {
	if _, err := fh.Seek(section.DataOffset, io.SeekStart); err != nil {
		return err
	}
	rd := make([]byte, section.Size)
	if _, err := fh.Read(rd); err != nil {
		return err
	}

	data, err := shared.DecompressZlib(rd)
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
			switch lineNum {
			case 0:
				ewfHeader.NofCategories = string(strAttr[0])
			case 1:
				ewfHeader.CategoryName = strAttr
			case 2:
				_, ok := AcquiredMediaIdentifiers[EWFMediaInfo(strAttr)]
				if !ok {
					return fmt.Errorf("media identifier is unknown: %s", strAttr)
				}
				identifiers = append(identifiers, strAttr)
			case 3:
				values = append(values, strAttr)
			}
		}
	}
	ewfHeader.MediaInfo = shared.ToMap(identifiers, values)

	return nil
}

func (ewfHeader *EWFHeaderSection) Encode(ewf io.WriteSeeker) error {
	buf := bytes.NewBuffer(nil)

	buf.WriteString(ewfHeader.NofCategories)
	buf.Write(newLineDelim)
	buf.WriteString(ewfHeader.CategoryName)
	buf.Write(newLineDelim)

	mk := make([]string, 0, len(ewfHeader.MediaInfo))
	mv := make([]string, 0, len(ewfHeader.MediaInfo))
	for k, v := range ewfHeader.MediaInfo {
		mk = append(mk, k)
		mv = append(mv, v)
	}
	buf.WriteString(strings.Join(mk, string(fieldDelim)))
	buf.Write(newLineDelim)
	buf.WriteString(strings.Join(mv, string(fieldDelim)))
	buf.Write(newLineDelim)

	comp, err := shared.NewZlibCompressor()
	if err != nil {
		return err
	}
	zlHeader, err := comp.Compress(buf.Bytes())
	if err != nil {
		return err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_HEADER)

	currentPosition, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	desc.Size = uint64(len(zlHeader)) + DescriptorSize
	desc.Next = desc.Size + uint64(currentPosition)

	// header section and its data appears twice subsequently
	// after first write, we arrange the "Next" field then write
	_, desc.Checksum, err = shared.WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}
	_, err = ewf.Write(zlHeader)
	if err != nil {
		return err
	}

	desc.Next = desc.Next + desc.Size

	_, _, err = shared.WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}

	_, err = ewf.Write(zlHeader)
	return err
}
