package evf1

import (
	"crypto/md5"
	"crypto/sha1"
	"hash"
	"io"
	"sync"

	"github.com/asalih/go-ewf/shared"
)

// EWFWriter is helper for creating E01 images. Data is always compressed
type EWFWriter struct {
	mu   sync.Mutex
	dest io.WriteSeeker

	dataSize uint64
	buf      []byte

	md5Hasher  hash.Hash
	sha1Hasher hash.Hash

	Segment       *EWFSegment
	SegmentOffset uint32
	ChunkSize     uint32
}

func CreateEWF(dest io.WriteSeeker) (*EWFWriter, error) {
	ewf := &EWFWriter{
		dest:          dest,
		buf:           make([]byte, 0, DefaultChunkSize),
		SegmentOffset: 0,
		ChunkSize:     0,
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

	ewf.Segment.Header = &EWFHeaderSection{}
	ewf.Segment.Header.CategoryName = "main"
	ewf.Segment.Header.NofCategories = "1"
	ewf.Segment.Header.MediaInfo = make(map[string]string)

	ewf.Segment.Sectors = new(EWFSectorsSection)
	ewf.Segment.Volume = &EWFVolumeSection{
		Data: DefaultVolume(),
	}

	ewf.Segment.Tables = []*EWFTableSection{
		newTable(),
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
	err := ewf.Segment.EWFHeader.Encode(ewf.dest)
	if err != nil {
		return err
	}

	err = ewf.Segment.Header.Encode(ewf.dest)
	if err != nil {
		return err
	}

	// Volume comes before data we put so default volume used as placeholder
	err = ewf.Segment.Volume.Encode(ewf.dest)
	if err != nil {
		return err
	}

	// sectors descriptor also comes before the data
	return ewf.Segment.Sectors.Encode(ewf.dest, 0, 0)
}

func (ewf *EWFWriter) Write(p []byte) (n int, err error) {
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
		ewf.buf = shared.PadBytes(ewf.buf, DefaultChunkSize)
		_, err := ewf.writeData(ewf.buf)
		if err != nil {
			ewf.mu.Unlock()
			return err
		}
		ewf.buf = ewf.buf[:0]
		ewf.mu.Unlock()
	}

	tablePosition, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	err = ewf.Segment.Sectors.Encode(ewf.dest, uint64(ewf.dataSize), uint64(tablePosition))
	if err != nil {
		return err
	}

	// volume will be saved in its poisition
	err = ewf.Segment.Volume.Encode(ewf.dest)
	if err != nil {
		return err
	}

	for _, tbl := range ewf.Segment.Tables {
		//table2 is basically a mirror of the table so the table encodes itself twice.
		err = tbl.Encode(ewf.dest)
		if err != nil {
			return err
		}
	}

	copy(ewf.Segment.Digest.MD5[:], ewf.md5Hasher.Sum(nil))
	copy(ewf.Segment.Digest.SHA1[:], ewf.sha1Hasher.Sum(nil))
	err = ewf.Segment.Digest.Encode(ewf.dest)
	if err != nil {
		return err
	}

	copy(ewf.Segment.Hash.MD5[:], ewf.md5Hasher.Sum(nil))
	err = ewf.Segment.Hash.Encode(ewf.dest)
	if err != nil {
		return err
	}

	ewf.Segment.Data.ChunkCount = ewf.Segment.Volume.Data.GetChunkCount()
	ewf.Segment.Data.Sectors = uint64(ewf.Segment.Volume.Data.GetSectorCount() * ewf.Segment.Volume.Data.GetChunkCount())
	err = ewf.Segment.Data.Encode(ewf.dest)
	if err != nil {
		return err
	}

	err = ewf.Segment.Done.Encode(ewf.dest)

	return err
}

// Seek implements vfs.FileDescriptionImpl.Seek.
func (ewf *EWFWriter) Seek(offset int64, whence int) (ret int64, err error) {
	return ewf.dest.Seek(offset, whence)
}

func (ewf *EWFWriter) writeData(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	position, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	var bufc []byte
	bufc, err = shared.CompressZlib(p)
	if err != nil {
		return
	}

	n, err = ewf.dest.Write(bufc)
	ewf.dataSize += uint64(n)
	if err != nil {
		return
	}

	ewf.Segment.addTableEntry(uint32(position))
	ewf.Segment.Volume.Data.IncrementChunkCount()

	_, err = ewf.md5Hasher.Write(p)
	if err != nil {
		return
	}

	_, err = ewf.sha1Hasher.Write(p)
	return
}
