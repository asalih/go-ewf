package evf2

import (
	"container/list"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/asalih/go-ewf/shared"
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

type EWFReader struct {
	First *EWFSegment

	ChunkSize uint32
	Size      int64

	decompressor shared.Decompressor
	segments     *list.List
	position     int64
}

func OpenEWF(fhs ...io.ReadSeeker) (*EWFReader, error) {
	ewf := &EWFReader{
		segments:  list.New(),
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
		return nil, errors.New("failed to load EWF")
	}

	ewf.First = allSegments[0]
	switch ewf.First.EWFHeader.CompressionMethod {
	case 0:
		ewf.decompressor = shared.SkipDecompress
	case 1:
		ewf.decompressor = shared.DecompressZlib
	case 2:
		ewf.decompressor = shared.DecompressBZip2
	default:
		return nil, fmt.Errorf("unsupported compression method: %v", ewf.First.EWFHeader.CompressionMethod)
	}

	err := ewf.First.Decode(nil, ewf.decompressor)
	if err != nil {
		return nil, err
	}

	for _, v := range allSegments {
		_ = ewf.segments.PushBack(v)
	}

	if ewf.First.DeviceInformation == nil {
		return nil, fmt.Errorf("failed to load EWF")
	}

	sc, err := ewf.First.CaseData.GetSectorCount()
	if err != nil {
		return nil, err
	}
	ss, err := ewf.First.DeviceInformation.GetSectorSize()
	if err != nil {
		return nil, err
	}

	ewf.ChunkSize = uint32(sc * ss)

	cc, err := ewf.First.CaseData.GetChunkCount()
	if err != nil {
		return nil, err
	}
	ewf.Size = int64(uint32(cc) * ewf.ChunkSize)

	return ewf, nil
}

func (ewf *EWFReader) Read(p []byte) (n int, err error) {
	n, err = ewf.ReadAt(p, ewf.position)
	ewf.position += int64(n)
	return
}

func (ewf *EWFReader) ReadAt(p []byte, off int64) (n int, err error) {
	sectorSize, err := ewf.First.DeviceInformation.GetSectorSize()
	if err != nil {
		return 0, err
	}
	sectorOffset := off / int64(sectorSize)
	length := len(p)
	sectorCount := (length + sectorSize - 1) / sectorSize

	buf, err := ewf.readSectors(uint32(sectorOffset), uint32(sectorCount))
	if err != nil {
		return 0, err
	}

	bufOff := off % int64(sectorSize)
	copyLength := min(uint32(len(buf)-int(bufOff)), uint32(length))
	n = copy(p, buf[bufOff:bufOff+int64(copyLength)])
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
	elem, ok := shared.GetListElement(ewf.segments, index)
	if !ok {
		return nil, nil, errors.New("not found")
	}

	seg := elem.Value.(*EWFSegment)
	if !seg.isDecoded {
		var prevSeg *EWFSegment
		if pe := elem.Prev(); pe != nil {
			prevSeg = pe.Value.(*EWFSegment)
		}
		err := seg.Decode(prevSeg, ewf.decompressor)
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
