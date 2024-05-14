package evf2

import (
	"encoding/binary"
	"fmt"
	"io"
)

type EWFSectionDescriptorData struct {
	Type           uint32
	DataFlags      uint32
	PreviousOffset uint64
	DataSize       uint64
	DescriptorSize uint32
	PaddingSize    uint32
	MD5Hash        [16]byte
	Pad            [12]byte
	Checksum       uint32
}

var DescriptorSize = int64(binary.Size(&EWFSectionDescriptorData{}))

func NewEWFSectionDescriptorData(secType EWFSectionType) *EWFSectionDescriptorData {
	return &EWFSectionDescriptorData{
		Type:           uint32(secType),
		DescriptorSize: uint32(DescriptorSize),
		MD5Hash:        [16]byte{},
		Pad:            [12]byte{},
	}
}

type EWFSectionDescriptor struct {
	fh     io.ReadSeeker
	offset int64

	Descriptor *EWFSectionDescriptorData
	Type       EWFSectionType
	Previous   uint64
	Size       uint64
	Checksum   uint32
	DataOffset int64
}

func NewEWFSectionDescriptor(fh io.ReadSeeker) (*EWFSectionDescriptor, error) {
	offset, err := fh.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	var descriptor EWFSectionDescriptorData
	if err := binary.Read(fh, binary.LittleEndian, &descriptor); err != nil {
		return nil, err
	}

	return &EWFSectionDescriptor{
		fh:         fh,
		offset:     offset,
		Descriptor: &descriptor,
		Type:       EWFSectionType(descriptor.Type),
		Previous:   descriptor.PreviousOffset,
		Size:       descriptor.DataSize,
		Checksum:   descriptor.Checksum,
		DataOffset: offset - int64(descriptor.DataSize),
	}, nil
}

func (esd *EWFSectionDescriptor) String() string {
	return fmt.Sprintf("<EWFSection type=%s size=0x%x offset=0x%x checksum=0x%x>", esd.Type, esd.Size, esd.offset, esd.Checksum)
}
