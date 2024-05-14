package evf1

import (
	"io"

	"github.com/asalih/go-ewf/shared"
)

type EWFDoneSection struct {
}

func (d *EWFDoneSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor) error {
	//done has no data
	return nil
}

func (d *EWFDoneSection) Encode(ewf io.WriteSeeker) (err error) {
	currentPosition, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_DONE)
	desc.Size = DescriptorSize
	desc.Next = uint64(currentPosition)
	_, _, err = shared.WriteWithSum(ewf, desc)
	return
}
