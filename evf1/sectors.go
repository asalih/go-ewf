package evf1

import (
	"io"

	"github.com/asalih/go-ewf/shared"
)

type EWFSectorsSection struct {
	position int64
}

func (d *EWFSectorsSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor) error {
	//sectors has no data
	return nil
}

func (d *EWFSectorsSection) Encode(ewf io.WriteSeeker, dataSize, next uint64) (err error) {
	currentPosition, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_SECTORS)

	desc.Size = dataSize + DescriptorSize
	desc.Next = next

	if d.position <= 0 {
		d.position = currentPosition
	} else {
		defer func() {
			_, errs := ewf.Seek(currentPosition, io.SeekStart)
			if errs != nil && err == nil {
				err = errs
			}
		}()
	}

	_, err = ewf.Seek(d.position, io.SeekStart)
	if err != nil {
		return err
	}

	_, desc.Checksum, err = shared.WriteWithSum(ewf, desc)
	return
}
