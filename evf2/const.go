package evf2

import "bytes"

const (
	EVF2Signature = "EVF2\x0d\x0a\x81\x00"
	LVF2Signature = "LVF2\x0d\x0a\x81\x00"
)

var (
	newLineDelim = []byte{'\n'}
	fieldDelim   = []byte{'\t'}
)

const (
	DefaultChunkSize = 32768
	ChecksumSize     = 4
	Uint32Size       = 4
)

const defaultMaxTableLength uint32 = 16375

// maxTableLength controls how many chunk entries go into a single table section.
// Kept as a var so tests can lower it to exercise multi-table behavior.
var maxTableLength uint32 = defaultMaxTableLength

type EWFSectionType uint32

const (
	EWF_SECTION_TYPE_DEVICE_INFORMATION   EWFSectionType = 1
	EWF_SECTION_TYPE_CASE_DATA            EWFSectionType = 2
	EWF_SECTION_TYPE_SECTOR_DATA          EWFSectionType = 3
	EWF_SECTION_TYPE_SECTOR_TABLE         EWFSectionType = 4
	EWF_SECTION_TYPE_ERROR_TABLE          EWFSectionType = 5
	EWF_SECTION_TYPE_SESSION_TABLE        EWFSectionType = 6
	EWF_SECTION_TYPE_INCREMENET_DATA      EWFSectionType = 7
	EWF_SECTION_TYPE_MD5_HASH             EWFSectionType = 8
	EWF_SECTION_TYPE_SHA1_HASH            EWFSectionType = 9
	EWF_SECTION_TYPE_RESTART_DATA         EWFSectionType = 10
	EWF_SECTION_TYPE_ENCRYPTION_KEYS      EWFSectionType = 11
	EWF_SECTION_TYPE_MEMORY_EXTENTS_TABLE EWFSectionType = 12
	EWF_SECTION_TYPE_NEXT                 EWFSectionType = 13
	EWF_SECTION_TYPE_FINAL_INFORMATION    EWFSectionType = 14
	EWF_SECTION_TYPE_DONE                 EWFSectionType = 15
	EWF_SECTION_TYPE_ANALYTICAL_DATA      EWFSectionType = 16
)

// String implements the Stringer interface for EWFSectionType
func (t EWFSectionType) String() string {
	switch t {
	case EWF_SECTION_TYPE_DEVICE_INFORMATION:
		return "device_information"
	case EWF_SECTION_TYPE_CASE_DATA:
		return "case_data"
	case EWF_SECTION_TYPE_SECTOR_DATA:
		return "sector_data"
	case EWF_SECTION_TYPE_SECTOR_TABLE:
		return "sector_table"
	case EWF_SECTION_TYPE_ERROR_TABLE:
		return "error_table"
	case EWF_SECTION_TYPE_SESSION_TABLE:
		return "session_table"
	case EWF_SECTION_TYPE_INCREMENET_DATA:
		return "increment_data"
	case EWF_SECTION_TYPE_MD5_HASH:
		return "md5_hash"
	case EWF_SECTION_TYPE_SHA1_HASH:
		return "sha1_hash"
	case EWF_SECTION_TYPE_RESTART_DATA:
		return "restart_data"
	case EWF_SECTION_TYPE_ENCRYPTION_KEYS:
		return "encryption_keys"
	case EWF_SECTION_TYPE_MEMORY_EXTENTS_TABLE:
		return "memory_extents_table"
	case EWF_SECTION_TYPE_NEXT:
		return "next"
	case EWF_SECTION_TYPE_FINAL_INFORMATION:
		return "final_information"
	case EWF_SECTION_TYPE_DONE:
		return "done"
	case EWF_SECTION_TYPE_ANALYTICAL_DATA:
		return "analytical_data"
	default:
		return "unknown"
	}
}

const (
	// The chunk data is compressed
	EWF_CHUNK_DATA_FLAG_IS_COMPRESSED = 0x00000001
	// The chunk data has a checksum
	EWF_CHUNK_DATA_FLAG_HAS_CHECKSUM = 0x00000002
	// The chunk data uses pattern fill
	EWF_CHUNK_DATA_FLAG_USES_PATTERN_FILL = 0x00000004
)

const (
	EWF_COMPRESSION_METHOD_NONE  = 0
	EWF_COMPRESSION_METHOD_ZLIB  = 1
	EWF_COMPRESSION_METHOD_BZIP2 = 2
)

func alignTo16Bytes(data []byte) ([]byte, int) {
	padding := calculatePadding(len(data))
	if padding > 0 {
		data = append(data, bytes.Repeat([]byte{0}, padding)...)
	}
	return data, padding
}

func alignSizeTo16Bytes(sz int) ([]byte, int) {
	padding := calculatePadding(sz)
	return make([]byte, padding), padding
}

func calculatePadding(sz int) int {
	return (16 - (sz % 16)) % 16
}
