package ewf

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

type WriterAtSeeker interface {
	io.Writer
	io.WriterAt
	io.Seeker
}

// EWFWriter is helper for creating E01 images. Data is always compressed
type EWFWriter struct {
	mu       sync.Mutex
	dest     io.WriterAt
	position int64

	sectorsPosition int64
	dataSize        int64
	buf             []byte

	Segment       *EWFSegment
	SegmentOffset uint32
	ChunkSize     uint32
	TotalWritten  int64
}

func CreateEWF(dest io.WriterAt) (*EWFWriter, error) {
	ewf := &EWFWriter{
		dest:          dest,
		buf:           make([]byte, 0, DefaultChunkSize),
		SegmentOffset: 0,
		ChunkSize:     0,
		TotalWritten:  0,
	}

	ewf.Segment = NewEWFSegment()
	ewf.Segment.ewfheader = &EWFHeader{
		FieldsStart:   1,
		SegmentNumber: 1, // TODO: this number increments for each file chunk like E0n
		FieldsEnd:     0,
	}
	copy(ewf.Segment.ewfheader.Signature[:], []byte(evfSig))

	err := ewf.Segment.ewfheader.Encode(ewf)
	if err != nil {
		return nil, err
	}

	ewf.Segment.Header = &EWFHeaderSection{}
	ewf.Segment.Header.CategoryName = "main"
	ewf.Segment.Header.NofCategories = "1"
	ewf.Segment.Header.MediaInfo = make(map[string]string)
	ewf.Segment.tableOffsets = []int64{0}

	ewf.Segment.Volume = &EWFVolumeSection{
		Data: DefaultVolume(),
	}

	ewf.Segment.Tables = []*EWFTableSection{
		{

			Header: &EWFTableSectionHeader{
				Entries: make([]uint32, 0),
			},
		},
	}

	return ewf, nil
}

func (ewf *EWFWriter) AddMediaInfo(key EWFMediaInfo, value string) {
	ewf.Segment.Header.MediaInfo[string(key)] = value
}

func (ewf *EWFWriter) Start() error {
	err := ewf.Segment.Header.Encode(ewf)
	if err != nil {
		return err
	}

	// Volume comes before data we put so default volume used as placeholder
	err = ewf.Segment.Volume.Encode(ewf)
	if err != nil {
		return err
	}

	// sectors descriptor also comes before the data
	return ewf.encodeSectorsDescriptor()
}

func (ewf *EWFWriter) Write(p []byte) (n int, err error) {
	ewf.mu.Lock()
	defer ewf.mu.Unlock()

	n, err = ewf.dest.WriteAt(p, ewf.position)
	ewf.position += int64(n)
	ewf.TotalWritten += int64(n)
	return
}

func (ewf *EWFWriter) WriteData(p []byte) (n int, err error) {
	ewf.mu.Lock()
	defer ewf.mu.Unlock()

	ewf.buf = append(ewf.buf, p...)
	n = len(p)

	if len(ewf.buf) < DefaultChunkSize {
		return
	}

	for len(ewf.buf) >= DefaultChunkSize {
		_, err = ewf.writeData(ewf.buf[:DefaultChunkSize])
		if err != nil {
			return
		}

		ewf.buf = ewf.buf[DefaultChunkSize:]

	}

	return
}

func (ewf *EWFWriter) Close() error {
	_, err := ewf.writeData(ewf.buf)
	if err != nil {
		return err
	}
	ewf.buf = ewf.buf[:0]

	ewf.Segment.tableOffsets[0] = ewf.position
	err = ewf.encodeSectorsDescriptor()
	if err != nil {
		return err
	}

	err = ewf.Segment.Volume.Encode(ewf)
	if err != nil {
		return err
	}

	err = ewf.Segment.Tables[0].Encode(ewf)
	if err != nil {
		return err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_DONE)
	desc.Size = uint64(binary.Size(desc))
	desc.Next = uint64(ewf.position)
	_, _, err = WriteWithSum(ewf, desc)
	return err
}

func (ewf *EWFWriter) writeData(p []byte) (n int, err error) {
	cpos := ewf.position

	//TODO:Documentation mention about Checksum for each chunk but its not working actually?
	// sum := adler32.Checksum(p)
	// p = binary.LittleEndian.AppendUint32(p, sum)

	//TODO(ahmet): Should I reuse zlib writer?
	var bufc []byte
	bufc, err = compress(p)
	if err != nil {
		return
	}

	n, err = ewf.dest.WriteAt(bufc, ewf.position)
	ewf.position += int64(n)
	ewf.dataSize += int64(n)

	ewf.TotalWritten += int64(n)

	//TODO: does the tables need to be slice?
	ewf.Segment.Tables[0].addEntry(uint32(cpos))
	ewf.Segment.Volume.Data.IncrementChunkCount()
	return
}

// Seek implements vfs.FileDescriptionImpl.Seek.
func (ewf *EWFWriter) Seek(offset int64, whence int) (ret int64, err error) {
	var newPos int64

	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = ewf.position + offset
	case io.SeekEnd:
		newPos = ewf.TotalWritten + offset
	default:
		return 0, errors.New("invalid whence value")
	}

	if newPos < 0 {
		return 0, errors.New("negative position")
	}

	ewf.position = newPos
	return newPos, nil
}

func (ewf *EWFWriter) encodeSectorsDescriptor() error {
	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_SECTORS)

	desc.Next = uint64(ewf.Segment.tableOffsets[0])
	desc.Size = uint64(ewf.dataSize)

	currentPosition := ewf.position
	if ewf.sectorsPosition <= 0 {
		ewf.sectorsPosition = currentPosition
	} else {
		defer func() {
			ewf.position = currentPosition
		}()
	}

	ewf.position = ewf.sectorsPosition

	_, _, err := WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}
	return nil
}
