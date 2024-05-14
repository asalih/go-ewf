package evf2

import (
	"encoding/binary"
	"io"

	"github.com/asalih/go-ewf/shared"
)

const sha1SectionPaddingSize = 8

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
	//alignment padding
	err = binary.Write(ewf, binary.LittleEndian, [sha1SectionPaddingSize]byte{})
	if err != nil {
		return 0, 0, nil
	}
	dataN += sha1SectionPaddingSize

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_SHA1_HASH)

	desc.DataSize = uint64(binary.Size(d))
	desc.PreviousOffset = uint64(previousDescriptorPosition)
	desc.DataFlags = EWF_CHUNK_DATA_FLAG_HAS_CHECKSUM

	descN, desc.Checksum, err = shared.WriteWithSum(ewf, desc)
	if err != nil {
		return 0, 0, err
	}

	return dataN, descN, nil
}
