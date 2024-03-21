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

type EWF struct {
	Segments       []*EWFSegment
	SegmentOffsets []uint32
	Header         *EWFHeaderSection
	Volume         *EWFVolumeSection
	ChunkSize      uint32
	Size           int64
}

func CreateEWF(dest io.WriterAt) (*EWF, error) {
	return nil, nil
}

func OpenEWF(fh io.ReadSeeker) (*EWF, error) {
	ewf := &EWF{
		Segments:       []*EWFSegment{},
		SegmentOffsets: []uint32{},
		Header:         nil,
		Volume:         nil,
		ChunkSize:      0,
		Size:           0,
	}

	segmentOffset := uint32(0)

	segment, err := NewEWFSegment(fh, ewf)
	if err != nil {
		return nil, err
	}

	if segment.header != nil && ewf.Header == nil {
		ewf.Header = segment.header
	}

	if segment.volume != nil && ewf.Volume == nil {
		ewf.Volume = segment.volume
	}

	if segmentOffset != 0 {
		ewf.SegmentOffsets = append(ewf.SegmentOffsets, segmentOffset)
	}

	segment.offset = int64(segmentOffset * ewf.Volume.SectorSize)
	segment.sectorOffset = int(segmentOffset)
	segmentOffset += uint32(segment.sectorCount)

	ewf.Segments = append(ewf.Segments, segment)

	if ewf.Header == nil || ewf.Volume == nil || len(ewf.Segments) == 0 {
		return nil, fmt.Errorf("failed to load EWF")
	}

	ewf.ChunkSize = ewf.Volume.SectorCount * ewf.Volume.SectorSize

	maxsize := int64(ewf.Volume.ChunkCount * ewf.Volume.SectorCount * ewf.Volume.SectorSize)
	lastTable := ewf.Segments[len(ewf.Segments)-1].tables[len(ewf.Segments[len(ewf.Segments)-1].tables)-1]

	dat, err := lastTable.readChunk(int64(lastTable.Header.NumEntries) - 1)
	if err != nil {
		return nil, err
	}
	lastChunkSize := int64(len(dat))

	ewf.Size = maxsize - (int64(ewf.ChunkSize) - lastChunkSize)

	return ewf, nil
}

func (ewf *EWF) ReadAt(p []byte, off int64) (n int, err error) {
	sectorOffset := off / int64(ewf.Volume.SectorSize)
	length := len(p)
	sectorCount := (length + int(ewf.Volume.SectorSize) - 1) / int(ewf.Volume.SectorSize)

	buf, err := ewf.readSectors(uint32(sectorOffset), uint32(sectorCount))
	if err != nil {
		return 0, err
	}

	bufOff := off % int64(ewf.Volume.SectorSize)
	n = copy(p, buf[bufOff:bufOff+int64(length)])
	return
}

func (ewf *EWF) WriteAt(p []byte, off int64) (n int, err error) {
	return 0, nil
}

func (ewf *EWF) readSectors(sector uint32, count uint32) ([]byte, error) {
	buf := make([]byte, 0)

	segmentIdx := sort.Search(len(ewf.SegmentOffsets), func(i int) bool {
		return ewf.SegmentOffsets[i] > sector
	})

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

		segmentIdx++
	}

	return buf, nil
}

func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
