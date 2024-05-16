package evf2

import (
	"encoding/binary"
	"io"

	"github.com/asalih/go-ewf/shared"
)

type EWFSHA1Section struct {
	Hash     [20]uint8
	Checksum uint32
}

func (d *EWFSHA1Section) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor) error {
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

func (d *EWFSHA1Section) Encode(ewf io.Writer, previousDescriptorPosition int64) (dataN int, descN int, err error) {
	dataN, d.Checksum, err = shared.WriteWithSum(ewf, d)
	if err != nil {
		return 0, 0, nil
	}

	pad, padSize := alignSizeTo16Bytes(dataN)
	_, err = ewf.Write(pad)
	if err != nil {
		return 0, 0, nil
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_SHA1_HASH)

	desc.DataSize = uint64(binary.Size(d) + padSize)
	desc.PreviousOffset = uint64(previousDescriptorPosition)
	desc.PaddingSize = uint32(padSize)

	descN, desc.Checksum, err = shared.WriteWithSum(ewf, desc)
	if err != nil {
		return 0, 0, err
	}

	return dataN, descN, nil
}
