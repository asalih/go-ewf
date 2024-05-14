package evf2

import (
	"io"

	"github.com/asalih/go-ewf/shared"
)

type EWFDoneSection struct {
}

func (d *EWFDoneSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) error {
	//done has no data
	return nil
}

func (d *EWFDoneSection) Encode(ewf io.Writer, previousDescriptorPosition int64) (dataN int, descN int, err error) {
	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_DONE)

	desc.DataSize = 0
	desc.PreviousOffset = uint64(previousDescriptorPosition)
	desc.DataFlags = EWF_CHUNK_DATA_FLAG_HAS_CHECKSUM

	descN, desc.Checksum, err = shared.WriteWithSum(ewf, desc)
	if err != nil {
		return 0, 0, err
	}

	return dataN, descN, nil
}
