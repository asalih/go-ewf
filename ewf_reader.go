package ewf

import (
	"container/list"
	"errors"
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
	segments *list.List
	First    *EWFSegment
	// SegmentOffsets []uint32

	ChunkSize uint32
	Size      int64

	position int64
}

func OpenEWF(fhs ...io.ReadSeeker) (*EWFReader, error) {
	ewf := &EWFReader{
		segments: list.New(),
		// SegmentOffsets: make([]uint32, 0),
		ChunkSize: 0,
		Size:      0,
	}

	allSegments := make([]*EWFSegment, 0)
	for _, file := range fhs {
		segment, err := NewEWFSegment(file)
		if err != nil {
			return nil, err
		}

		allSegments = append(allSegments, segment)
	}

	sort.SliceStable(allSegments, func(i, j int) bool {
		return allSegments[i].EWFHeader.SegmentNumber < allSegments[j].EWFHeader.SegmentNumber
	})

	if len(allSegments) == 0 {
		return nil, fmt.Errorf("failed to load EWF")
	}

	ewf.First = allSegments[0]
	err := ewf.First.Decode(nil)
	if err != nil {
		return nil, err
	}

	for _, v := range allSegments {
		_ = ewf.segments.PushBack(v)
	}

	// var segmentOffset uint32
	// for i, segment := range ewf.Segments {
	// 	ts := time.Now()
	// 	// the table in the segment requires volume for calculations so needed to pass it from the first segment
	// 	err := segment.Decode(ewf.First.Volume)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	fmt.Printf("idx: %v, time: %s\n", i, time.Since(ts))

	// 	if segmentOffset != 0 {
	// 		ewf.SegmentOffsets = append(ewf.SegmentOffsets, segmentOffset)
	// 	}
	// 	segment.sectorOffset = int(segmentOffset)
	// 	segmentOffset += uint32(segment.sectorCount)
	// }

	if ewf.First.Header == nil || ewf.First.Volume == nil {
		return nil, fmt.Errorf("failed to load EWF")
	}

	ewf.ChunkSize = ewf.First.Volume.Data.GetSectorCount() * ewf.First.Volume.Data.GetSectorSize()
	ewf.Size = int64(ewf.First.Volume.Data.GetChunkCount() * ewf.ChunkSize)

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

// Seek implements vfs.FileDescriptionImpl.Seek.
func (ewf *EWFReader) Seek(offset int64, whence int) (ret int64, err error) {
	var newPos int64

	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = ewf.position + offset
	case io.SeekEnd:
		newPos = ewf.Size + offset
	default:
		return 0, errors.New("invalid whence value")
	}

	if newPos < 0 {
		return 0, errors.New("negative position")
	}

	ewf.position = newPos
	return newPos, nil
}

func (ewf *EWFReader) Segment(index int) (*EWFSegment, *list.Element, error) {
	elem, ok := getElementAtIndex(ewf.segments, index)
	if !ok {
		return nil, nil, errors.New("not found")
	}

	seg := elem.Value.(*EWFSegment)
	if !seg.isDecoded {
		var prevSeg *EWFSegment
		if pe := elem.Prev(); pe != nil {
			prevSeg = pe.Value.(*EWFSegment)
		}
		err := seg.Decode(prevSeg)
		if err != nil {
			return nil, nil, err
		}
	}
	return seg, elem, nil
}

func (ewf *EWFReader) calculateIndex(sector uint32) (int, error) {
	for i := 0; i < ewf.segments.Len(); i++ {
		seg, _, err := ewf.Segment(i)
		if err != nil {
			return 0, err
		}
		if int(sector) > seg.sectorOffset+seg.sectorCount {
			continue
		}
		return i, nil
	}
	return 0, fmt.Errorf("sector too long: %v", sector)
}

func (ewf *EWFReader) readSectors(sector uint32, count uint32) ([]byte, error) {
	buf := make([]byte, 0)

	segmentIdx, err := ewf.calculateIndex(sector)
	if err != nil {
		return nil, err
	}

	for count > 0 {
		if segmentIdx >= ewf.segments.Len() {
			return buf, io.EOF
		}
		segment, _, err := ewf.Segment(segmentIdx)
		if err != nil {
			return nil, err
		}
		segmentRemainingSectors := uint32(segment.sectorCount) - (sector - uint32(segment.sectorOffset))
		segmentSectors := min(segmentRemainingSectors, count)
		if segmentSectors <= 0 {
			return buf, io.EOF
		}

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
