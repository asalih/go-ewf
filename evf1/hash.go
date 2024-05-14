package evf1

import (
	"encoding/binary"
	"io"

	"github.com/asalih/go-ewf/shared"
)

type EWFHashSection struct {
	MD5      [16]uint8
	Unknown  [16]uint8
	Checksum uint32
}

func (d *EWFHashSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor) error {
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

	desc.Size = uint64(binary.Size(d)) + DescriptorSize
	desc.Next = desc.Size + uint64(currentPosition)

	_, desc.Checksum, err = shared.WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}

	_, d.Checksum, err = shared.WriteWithSum(ewf, d)
	return err
}
