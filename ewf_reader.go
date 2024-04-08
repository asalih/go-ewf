package ewf

import (
	"fmt"
	"io"
)

type MediaType uint8

const (
	Removable MediaType = 0x00
	Fixed     MediaType = 0x01
	Optical   MediaType = 0x03
	Logical   MediaType = 0x0e
	RAM       MediaType = 0x10
)

type MediaFlags uint8

const (
	Image    MediaFlags = 0x01
	Physical MediaFlags = 0x02
	Fastbloc MediaFlags = 0x04
	Tablaeu  MediaFlags = 0x08
)

type CompressionLevel uint8

const (
	None CompressionLevel = 0x00
	Good CompressionLevel = 0x01
	Best CompressionLevel = 0x02
)

type EWFReader struct {
	Segment       *EWFSegment
	SegmentOffset uint32
	ChunkSize     uint32
	Size          int64
}

func OpenEWF(fh io.ReadSeeker) (*EWFReader, error) {
	ewf := &EWFReader{
		SegmentOffset: 0,
		ChunkSize:     0,
		Size:          0,
	}

	segment := NewEWFSegment()
	err := segment.Decode(fh)
	if err != nil {
		return nil, err
	}
	if segment.Header == nil || segment.Volume == nil {
		return nil, fmt.Errorf("failed to load EWF")
	}

	ewf.Segment = segment

	ewf.ChunkSize = ewf.Segment.Volume.Data.GetSectorCount() * ewf.Segment.Volume.Data.GetSectorSize()

	maxsize := int64(ewf.Segment.Volume.Data.GetChunkCount() * ewf.Segment.Volume.Data.GetSectorCount() * ewf.Segment.Volume.Data.GetSectorSize())
	lastTable := ewf.Segment.Tables[len(ewf.Segment.Tables)-1]

	dat, err := lastTable.readChunk(int64(lastTable.Header.NumEntries) - 1)
	if err != nil {
		return nil, err
	}
	lastChunkSize := int64(len(dat))

	ewf.Size = maxsize - (int64(ewf.ChunkSize) - lastChunkSize)

	return ewf, nil
}

func (ewf *EWFReader) ReadAt(p []byte, off int64) (n int, err error) {
	sectorOffset := off / int64(ewf.Segment.Volume.Data.GetSectorSize())
	length := len(p)
	sectorCount := (length + int(ewf.Segment.Volume.Data.GetSectorSize()) - 1) / int(ewf.Segment.Volume.Data.GetSectorSize())

	buf, err := ewf.readSectors(uint32(sectorOffset), uint32(sectorCount))
	if err != nil {
		return 0, err
	}

	bufOff := off % int64(ewf.Segment.Volume.Data.GetSectorSize())
	n = copy(p, buf[bufOff:bufOff+int64(length)])
	return
}

func (ewf *EWFReader) readSectors(sector uint32, count uint32) ([]byte, error) {
	buf := make([]byte, 0)

	for count > 0 {
		segmentRemainingSectors := uint32(ewf.Segment.sectorCount) - (sector - uint32(ewf.Segment.sectorOffset))
		segmentSectors := min(segmentRemainingSectors, count)

		dat, err := ewf.Segment.ReadSectors(int64(sector), int(segmentSectors))
		if err != nil {
			return nil, err
		}
		buf = append(buf, dat...)
		sector += segmentSectors
		count -= segmentSectors
	}

	return buf, nil
}

func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
