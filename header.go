package ewf

import (
	"bytes"
	"encoding/binary"
	"io"
	"unicode/utf16"
)

type EWFHeaderSection struct {
	Data string
}

func NewEWFHeaderSection(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) (*EWFHeaderSection, error) {
	fh.Seek(section.DataOffset, io.SeekStart)
	rd := make([]byte, section.Size)
	if _, err := fh.Read(rd); err != nil {
		return nil, err
	}

	var res string
	if rd[0] == '\xff' || rd[0] == '\xfe' {
		u16s := make([]uint16, len(rd)/2)
		binary.Read(bytes.NewReader(rd), binary.LittleEndian, &u16s)
		res = string(utf16.Decode(u16s))
	} else {
		res = string(rd)
	}

	return &EWFHeaderSection{Data: res}, nil
}
