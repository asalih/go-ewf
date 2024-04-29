package ewf

import (
	"crypto/md5"
	"crypto/sha1"
	"errors"
	"hash"
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

	dataSize int64
	buf      []byte

	md5Hasher  hash.Hash
	sha1Hasher hash.Hash

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

	var err error
	ewf.Segment, err = NewEWFSegment(nil)
	if err != nil {
		return nil, err
	}
	ewf.Segment.EWFHeader = &EWFHeader{
		FieldsStart:   1,
		SegmentNumber: 1, // TODO: this number increments for each file chunk like E0n
		FieldsEnd:     0,
	}
	copy(ewf.Segment.EWFHeader.Signature[:], []byte(EVFSignature))

	err = ewf.Segment.EWFHeader.Encode(ewf)
	if err != nil {
		return nil, err
	}

	ewf.Segment.Header = &EWFHeaderSection{}
	ewf.Segment.Header.CategoryName = "main"
	ewf.Segment.Header.NofCategories = "1"
	ewf.Segment.Header.MediaInfo = make(map[string]string)

	ewf.Segment.Sectors = new(EWFSectorsSection)
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

	ewf.Segment.Digest = new(EWFDigestSection)
	ewf.Segment.Hash = new(EWFHashSection)
	ewf.Segment.Data = &EWFDataSection{
		MediaType:      1,
		MediaFlags:     1,
		SectorPerChunk: ewf.Segment.Volume.Data.GetSectorCount(),
		BytesPerSector: ewf.Segment.Volume.Data.GetSectorSize(),
	}

	ewf.Segment.Done = new(EWFDoneSection)

	ewf.md5Hasher = md5.New()
	ewf.sha1Hasher = sha1.New()

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
	return ewf.Segment.Sectors.Encode(ewf, 0, 0)
}

func (ewf *EWFWriter) Write(p []byte) (n int, err error) {
	ewf.mu.Lock()
	defer ewf.mu.Unlock()

	n, err = ewf.dest.WriteAt(p, ewf.position)
	ewf.position += int64(n)
	if ewf.position >= ewf.TotalWritten {
		ewf.TotalWritten += int64(n)
	}
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
	if len(ewf.buf) > 0 {
		ewf.mu.Lock()
		_, err := ewf.writeData(ewf.buf)
		if err != nil {
			ewf.mu.Unlock()
			return err
		}
		ewf.buf = ewf.buf[:0]
		ewf.mu.Unlock()
	}

	ewf.Segment.Tables[0].Offset = ewf.position
	err := ewf.Segment.Sectors.Encode(ewf, uint64(ewf.dataSize), uint64(ewf.Segment.Tables[0].Offset))
	if err != nil {
		return err
	}

	err = ewf.Segment.Volume.Encode(ewf)
	if err != nil {
		return err
	}

	//table2 is basically a mirror of the table so the table encodes itself twice.
	err = ewf.Segment.Tables[0].Encode(ewf)
	if err != nil {
		return err
	}

	copy(ewf.Segment.Digest.MD5[:], ewf.md5Hasher.Sum(nil))
	copy(ewf.Segment.Digest.SHA1[:], ewf.sha1Hasher.Sum(nil))
	err = ewf.Segment.Digest.Encode(ewf)
	if err != nil {
		return err
	}

	copy(ewf.Segment.Hash.MD5[:], ewf.md5Hasher.Sum(nil))
	err = ewf.Segment.Hash.Encode(ewf)
	if err != nil {
		return err
	}

	ewf.Segment.Data.ChunkCount = ewf.Segment.Volume.Data.GetChunkCount()
	ewf.Segment.Data.Sectors = uint64(ewf.Segment.Volume.Data.GetSectorCount() * ewf.Segment.Volume.Data.GetChunkCount())
	err = ewf.Segment.Data.Encode(ewf)
	if err != nil {
		return err
	}

	err = ewf.Segment.Done.Encode(ewf)

	return err
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
		newPos = ewf.dataSize + offset
	default:
		return 0, errors.New("invalid whence value")
	}

	if newPos < 0 {
		return 0, errors.New("negative position")
	}

	ewf.position = newPos
	return newPos, nil
}

func (ewf *EWFWriter) writeData(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	cpos := ewf.position
	var bufc []byte
	bufc, err = compress(p)
	if err != nil {
		return
	}

	n, err = ewf.dest.WriteAt(bufc, ewf.position)
	ewf.position += int64(n)
	ewf.dataSize += int64(n)
	if err != nil {
		return
	}

	ewf.TotalWritten += int64(n)

	ewf.Segment.Tables[0].addEntry(uint32(cpos))
	ewf.Segment.Volume.Data.IncrementChunkCount()

	_, err = ewf.md5Hasher.Write(p)
	if err != nil {
		return
	}

	_, err = ewf.sha1Hasher.Write(p)
	return
}
