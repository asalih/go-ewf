package ewf

import (
	"encoding/binary"
	"io"
)

type EWFHashSection struct {
	MD5      [16]uint8
	Unknown  uint8
	Checksum uint32
}

func (d *EWFHashSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) error {
	_, err := fh.Seek(section.DataOffset, io.SeekStart)
	if err != nil {
		return err
	}

	err = binary.Read(fh, binary.LittleEndian, d)
	if err != nil {
		return err
	}

	return nil
}

func (d *EWFHashSection) Encode(ewf io.WriteSeeker) error {
	currentPosition, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_HASH)

	dataSize := uint64(binary.Size(d))

	desc.Size = dataSize
	desc.Next = dataSize + DescriptorSize + uint64(currentPosition)

	_, desc.Checksum, err = WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}

	_, d.Checksum, err = WriteWithSum(ewf, d)
	return err
}
