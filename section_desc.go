package ewf

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

type EWFSectionDescriptorData struct {
	Type     [16]byte
	Next     uint64
	Size     uint64
	Pad      [40]byte
	Checksum uint32
}

type EWFSectionDescriptor struct {
	fh         io.ReadSeeker
	segment    *EWFSegment
	Descriptor *EWFSectionDescriptorData
	offset     int64
	Type       string
	Next       uint64
	Size       uint64
	Checksum   uint32
	DataOffset int64
}

func NewEWFSectionDescriptor(fh io.ReadSeeker, segment *EWFSegment) (*EWFSectionDescriptor, error) {
	offset, err := fh.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	var descriptor EWFSectionDescriptorData
	if err := binary.Read(fh, binary.LittleEndian, &descriptor); err != nil {
		return nil, err
	}

	size := uint64(descriptor.Size) - 0x4C
	dataOffset, _ := fh.Seek(0, io.SeekCurrent)

	return &EWFSectionDescriptor{
		fh:         fh,
		segment:    segment,
		offset:     offset,
		Type:       strings.TrimRight(string(descriptor.Type[:]), "\x00"),
		Next:       descriptor.Next,
		Size:       size,
		Checksum:   descriptor.Checksum,
		DataOffset: dataOffset,
	}, nil
}

func (esd *EWFSectionDescriptor) String() string {
	return fmt.Sprintf("<EWFSection type=%s size=0x%x offset=0x%x checksum=0x%x>", esd.Type, esd.Size, esd.offset, esd.Checksum)
}
