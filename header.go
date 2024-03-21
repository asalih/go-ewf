package ewf

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"io"
	"log"
)

const (
	EWF_HEADER_VALUES_INDEX_DESCRIPTION = iota
	EWF_HEADER_VALUES_INDEX_CASE_NUMBER
	EWF_HEADER_VALUES_INDEX_EXAMINER_NAME
	EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER
	EWF_HEADER_VALUES_INDEX_NOTES
	EWF_HEADER_VALUES_INDEX_ACQUIRY_SOFTWARE_VERSION
	EWF_HEADER_VALUES_INDEX_ACQUIRY_OPERATING_SYSTEM
	EWF_HEADER_VALUES_INDEX_ACQUIRY_DATE
	EWF_HEADER_VALUES_INDEX_SYSTEM_DATE
	EWF_HEADER_VALUES_INDEX_PASSWORD
	EWF_HEADER_VALUES_INDEX_PROCESS_IDENTIFIER
	EWF_HEADER_VALUES_INDEX_UNKNOWN_DC
	EWF_HEADER_VALUES_INDEX_EXTENTS
	EWF_HEADER_VALUES_INDEX_COMPRESSION_TYPE

	EWF_HEADER_VALUES_INDEX_MODEL
	EWF_HEADER_VALUES_INDEX_SERIAL_NUMBER
	EWF_HEADER_VALUES_INDEX_DEVICE_LABEL

	/* Value to indicate the default number of header values
	 */
	EWF_HEADER_VALUES_DEFAULT_AMOUNT
)

var CompressionLevels = map[string]string{
	"b": "Best",
	"f": "Fastest",
	"n": "No compression",
}

var AcquiredMediaIdentifiers = map[string]string{
	"a":   "Description",
	"c":   "Case Number",
	"n":   "Evidence Number",
	"e":   "Examiner Name",
	"t":   "Notes",
	"md":  "Media model",
	"sn":  "Serial number",
	"l":   "Device label",
	"an":  "AN unknown",
	"av":  "Version",
	"ov":  "Platform",
	"m":   "Acquired Date",
	"u":   "System Date",
	"p":   "Password Hash",
	"pid": "Process Identifiers",
	"dc":  "Unknown",
	"ext": "Extents",
	"r":   "Compression level",
}

type EWFHeaderSection struct {
	NofCategories     string
	CategoryName      string
	AcquiredMediaInfo map[string]string
}

func NewEWFHeaderSection(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) (*EWFHeaderSection, error) {
	fh.Seek(section.DataOffset, io.SeekStart)
	rd := make([]byte, section.Size)
	if _, err := fh.Read(rd); err != nil {
		return nil, err
	}

	val := Decompress(rd)

	//defer Utils.TimeTrack(time.Now(), "Parsing Header Section")
	line_del, _ := hex.DecodeString("0a")
	tab_del, err := hex.DecodeString("09")
	if err != nil {
		log.Fatal(err)
	}

	var identifiers []string
	var values []string

	ewf_h_section := EWFHeaderSection{}

	var time_ids []int // save ids of m & u
	for line_number, line := range bytes.Split(val, line_del) {
		for id_num, attr := range bytes.Split(line, tab_del) {

			if line_number == 0 {
				ewf_h_section.NofCategories = string(attr[0])

			} else if line_number == 1 {
				ewf_h_section.CategoryName = string(attr)
			} else if line_number == 2 {
				identifier := AcquiredMediaIdentifiers[string(attr)]
				if identifier == "Acquired Date" || identifier == "System Date" {
					time_ids = append(time_ids, id_num)
				}
				identifiers = append(identifiers, identifier)
			} else if line_number == 3 {
				if len(time_ids) == 2 && (id_num == time_ids[0] || id_num == time_ids[1]) {
					// values = append(values, Utils.GetTime(attr).Format("2006-01-02T15:04:05"))

				} else {
					values = append(values, string(attr))
				}

			}
		}
	}
	// ewf_h_section.AcquiredMediaInfo = Utils.ToMap(identifiers, values)

	// var res string
	// if rd[0] == '\xff' || rd[0] == '\xfe' {
	// 	u16s := make([]uint16, len(rd)/2)
	// 	binary.Read(bytes.NewReader(rd), binary.LittleEndian, &u16s)
	// 	res = string(utf16.Decode(u16s))
	// } else {
	// 	res = string(rd)
	// }

	// return &EWFHeaderSection{Data: res}, nil
	return &ewf_h_section, nil
}

/*CMF|FLG  0x78|  (FLG|CM)
CM 0-3 Compression method  8=deflate
CINFO 4-7 Compression info 7=32K window size only when CM=8
FLG 0-4 FCHECK  = CMF*256 + FLG multiple of 31 = 120*256==x mod 31 => x=156
5 FDICT 1=> DICT follows (DICT is the Adler-32 checksum  of this sequence of bytes )
6-7 FLEVEL compression level 0-3
9c = 1001 1100
FLEVEL 10
FDICT 0
FCHECK 12
ADLER32  algorithm is a 32-bit extension and improvement of the Fletcher algorithm,
A compliant decompressor must check CMF, FLG, and ADLER32,
*/

func Decompress(val []byte) []byte {
	//	defer TimeTrack(time.Now(), "decompressing")
	var r io.ReadCloser
	var b *bytes.Reader
	var bytesRead, lent int64
	var dst bytes.Buffer
	var err error

	b = bytes.NewReader(val)

	r, err = zlib.NewReader(b)
	if err != nil {
		if err == io.EOF {
			fmt.Println(err)

		}

		log.Fatal(err)
	}

	defer r.Close()

	lent, err = dst.ReadFrom(r)
	bytesRead += lent
	//	fmt.Println(":EM", bytesRead, len(val), int(bytesRead) > len(val))
	if err != nil {
		//fmt.Println(err)
		log.Fatal(err)

	}

	//var buf bytes.Buffer // buffer needs no initilization pointer
	if err != nil {
		panic(err)
	}

	//io.Copy(&buf, r)

	if err != nil {
		panic(err)
	}

	//   fmt.Printf("data  %d \n", len(buf.Bytes()))

	return dst.Bytes()
}
