package ewf

import (
	"fmt"
	"io"
	"sort"
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
	Segments      []*EWFSegment
	First         *EWFSegment
	SegmentOffset uint32
	ChunkSize     uint32
	Size          int64

	position int64
}

func OpenEWF(fhs ...io.ReadSeeker) (*EWFReader, error) {
	ewf := &EWFReader{
		Segments:      make([]*EWFSegment, 0),
		SegmentOffset: 0,
		ChunkSize:     0,
		Size:          0,
	}

	for _, file := range fhs {
		segment, err := NewEWFSegment(file)
		if err != nil {
			return nil, err
		}

		ewf.Segments = append(ewf.Segments, segment)
	}

	sort.SliceStable(ewf.Segments, func(i, j int) bool {
		return ewf.Segments[i].EWFHeader.SegmentNumber < ewf.Segments[j].EWFHeader.SegmentNumber
	})

	if len(ewf.Segments) == 0 {
		return nil, fmt.Errorf("failed to load EWF")
	}

	ewf.First = ewf.Segments[0]

	segmentOffset := 0
	for _, segment := range ewf.Segments {
		// the table in the segment requires volume for calculations so needed to pass it from the first segment
		err := segment.Decode(ewf.First.Volume)
		if err != nil {
			return nil, err
		}
		segmentOffset += segment.sectorCount
		segment.segmentOffset = segmentOffset
	}

	if ewf.First.Header == nil || ewf.First.Volume == nil {
		return nil, fmt.Errorf("failed to load EWF")
	}

	ewf.ChunkSize = ewf.First.Volume.Data.GetSectorCount() * ewf.First.Volume.Data.GetSectorSize()

	maxSize := ewf.First.Volume.Data.GetChunkCount() *
		ewf.First.Volume.Data.GetSectorCount() *
		ewf.First.Volume.Data.GetSectorSize()

	lastTable := ewf.Segments[len(ewf.Segments)-1].Table
	dat, err := lastTable.readChunk(int64(lastTable.Header.NumEntries) - 1)
	if err != nil {
		return nil, err
	}
	lastChunkSize := int64(len(dat))

	ewf.Size = int64(maxSize) - (int64(ewf.ChunkSize) - lastChunkSize)

	return ewf, nil
}

func (ewf *EWFReader) Read(p []byte) (n int, err error) {
	n, err = ewf.ReadAt(p, ewf.position)
	ewf.position += int64(n)
	return
}

func (ewf *EWFReader) ReadAt(p []byte, off int64) (n int, err error) {
	sectorSize := int(ewf.First.Volume.Data.GetSectorSize())
	sectorOffset := off / int64(sectorSize)
	length := len(p)
	sectorCount := (length + sectorSize - 1) / sectorSize

	buf, err := ewf.readSectors(uint32(sectorOffset), uint32(sectorCount))
	if err != nil {
		return 0, err
	}

	bufOff := off % int64(ewf.First.Volume.Data.GetSectorSize())
	n = copy(p, buf[bufOff:bufOff+int64(length)])
	return
}

func (ewf *EWFReader) readSectors(sector uint32, count uint32) ([]byte, error) {
	buf := make([]byte, 0)

	segmentIdx := sort.Search(len(ewf.Segments), func(i int) bool {
		return uint32(ewf.Segments[i].segmentOffset) > sector
	})
	if segmentIdx >= len(ewf.Segments) {
		return buf, io.EOF
	}

	for count > 0 {
		segment := ewf.Segments[segmentIdx]

		segmentRemainingSectors := uint32(segment.sectorCount) - (sector - uint32(segment.sectorOffset))
		segmentSectors := min(segmentRemainingSectors, count)

		dat, err := segment.ReadSectors(int64(sector), int(segmentSectors))
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
