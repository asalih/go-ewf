package ewf

import (
	"io"
)

type EWFDoneSection struct {
}

func (d *EWFDoneSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) error {
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
	_, _, err = WriteWithSum(ewf, desc)
	return
}
