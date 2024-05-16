package evf2

import (
	"io"

	"github.com/asalih/go-ewf/shared"
)

type EWFSectorsSection struct {
}

func (d *EWFSectorsSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor) error {
	//sectors has no data
	return nil
}

func (d *EWFSectorsSection) Encode(ewf io.Writer, dataSize uint64, paddingSize uint32, previousDescriptorPosition int64) (dataN int, descN int, err error) {
	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_SECTOR_DATA)

	desc.DataSize = dataSize + uint64(paddingSize)
	desc.PreviousOffset = uint64(previousDescriptorPosition)
	desc.PaddingSize = paddingSize

	descN, desc.Checksum, err = shared.WriteWithSum(ewf, desc)
	if err != nil {
		return 0, 0, err
	}

	return dataN, descN, nil
}
