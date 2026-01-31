package evf1

const (
	EVFSignature = "EVF\x09\x0d\x0a\xff\x00"
	LVFSignature = "LVF\x09\x0d\x0a\xff\x00"
)

var (
	newLineDelim = []byte{'\n'}
	fieldDelim   = []byte{'\t'}
)

const defaultMaxTableLength uint32 = 16375

// maxTableLength controls how many chunk entries go into a single table section.
// Kept as a var so tests can lower it to exercise multi-table behavior.
var maxTableLength uint32 = defaultMaxTableLength

const (
	DefaultChunkSize = 32768
	ChecksumSize     = 4
	Uint32Size       = 4
)

const (
	EWF_SECTION_TYPE_HEADER  = "header"
	EWF_SECTION_TYPE_HEADER2 = "header2"
	EWF_SECTION_TYPE_VOLUME  = "volume"
	EWF_SECTION_TYPE_DISK    = "disk"
	EWF_SECTION_TYPE_TABLE   = "table"
	EWF_SECTION_TYPE_TABLE2  = "table2"
	EWF_SECTION_TYPE_DATA    = "data"
	EWF_SECTION_TYPE_SECTORS = "sectors"
	EWF_SECTION_TYPE_ERRORS2 = "errors2"
	EWF_SECTION_TYPE_NEXT    = "next"
	EWF_SECTION_TYPE_SESSION = "session"
	EWF_SECTION_TYPE_HASH    = "hash"
	EWF_SECTION_TYPE_DIGEST  = "digest"
	EWF_SECTION_TYPE_DONE    = "done"
)
