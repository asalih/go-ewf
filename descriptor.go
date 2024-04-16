package ewf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type EWFSectionDescriptorData struct {
	Type     [16]byte
	Next     uint64
	Size     uint64
	Pad      [40]byte
	Checksum uint32
}

var DescriptorSize = uint64(binary.Size(&EWFSectionDescriptorData{}))

func NewEWFSectionDescriptorData(typeStr string) *EWFSectionDescriptorData {
	desc := EWFSectionDescriptorData{
		Pad: [40]byte{},
	}
	copy(desc.Type[:], typeStr)
	return &desc
}

type EWFSectionDescriptor struct {
	fh     io.ReadSeeker
	offset int64

	Descriptor *EWFSectionDescriptorData
	Type       string
	Next       uint64
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

	dataOffset, _ := fh.Seek(0, io.SeekCurrent)

	return &EWFSectionDescriptor{
		fh:         fh,
		offset:     offset,
		Descriptor: &descriptor,
		Type:       string(bytes.TrimRight(descriptor.Type[:], "\x00")),
		Next:       descriptor.Next,
		Size:       uint64(descriptor.Size - DescriptorSize),
		Checksum:   descriptor.Checksum,
		DataOffset: dataOffset,
	}, nil
}

func (esd *EWFSectionDescriptor) String() string {
	return fmt.Sprintf("<EWFSection type=%s size=0x%x offset=0x%x checksum=0x%x>", esd.Type, esd.Size, esd.offset, esd.Checksum)
}
