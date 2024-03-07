package ewf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
)

type EWFHeader struct {
	Signature     [8]byte
	FieldsStart   uint8
	SegmentNumber uint16
	FieldsEnd     uint16
}

type EWFSegment struct {
	fh           io.ReadSeeker
	ewf          *EWF
	ewfheader    *EWFHeader
	header       *EWFHeaderSection
	volume       *EWFVolumeSection
	sections     []*EWFSectionDescriptor
	tables       []*EWFTableSection
	tableOffsets []int64
	chunkCount   int
	sectorCount  int
	sectorOffset int
	size         int
	offset       int64
}

func NewEWFSegment(fh io.ReadSeeker, ewf *EWF) (*EWFSegment, error) {
	var ewfHeader EWFHeader
	err := binary.Read(fh, binary.LittleEndian, &ewfHeader)
	if err != nil {
		return nil, err
	}

	header := ewf.Header
	volume := ewf.Volume

	sig := string(ewfHeader.Signature[:])
	if sig != "EVF\x09\x0d\x0a\xff\x00" && sig != "LVF\x09\x0d\x0a\xff\x00" {
		return nil, fmt.Errorf("invalid signature, got %v", ewfHeader.Signature)
	}

	seg := &EWFSegment{
		fh:           fh,
		ewf:          ewf,
		ewfheader:    &ewfHeader,
		header:       header,
		volume:       volume,
		sections:     make([]*EWFSectionDescriptor, 0),
		tables:       make([]*EWFTableSection, 0),
		tableOffsets: make([]int64, 0),
		chunkCount:   0,
		sectorCount:  0,
		sectorOffset: 0,
		size:         0,
		offset:       0,
	}

	offset := int64(0)
	sectorOffset := int64(0)

	for {
		section, err := NewEWFSectionDescriptor(fh, seg)
		if err != nil {
			return nil, err
		}

		seg.sections = append(seg.sections, section)

		if section.Type == "header" || section.Type == "header2" && seg.header == nil {
			h, err := NewEWFHeaderSection(fh, section, seg)
			if err != nil {
				return nil, err
			}
			seg.header = h
		}

		if section.Type == "disk" || section.Type == "volume" && seg.volume == nil {
			v, err := NewEWFVolumeSection(fh, section, seg)
			if err != nil {
				return nil, err
			}
			seg.volume = v
		}

		if section.Type == "table" {
			table, err := NewEWFTableSection(fh, section, seg)
			if err != nil {
				return nil, err
			}

			if sectorOffset != 0 {
				seg.tableOffsets = append(seg.tableOffsets, sectorOffset)
			}

			table.Offset = sectorOffset * int64(seg.volume.SectorSize)
			table.SectorOffset = sectorOffset
			sectorOffset += table.SectorCount

			seg.tables = append(seg.tables, table)
		}

		if section.Next == uint64(offset) || section.Type == "done" {
			break
		}

		offset = int64(section.Next)
		fh.Seek(offset, io.SeekStart)
	}

	for _, t := range seg.tables {
		seg.chunkCount += int(t.Header.NumEntries)
	}

	seg.sectorCount = seg.chunkCount * int(seg.volume.SectorCount)
	seg.sectorOffset = -1
	seg.size = seg.chunkCount * int(seg.volume.SectorCount) * int(seg.volume.SectorSize)
	seg.offset = -1

	return seg, nil
}

func (seg *EWFSegment) ReadSectors(sector int64, count int) ([]byte, error) {
	// log.Debugf("EWFSegment::read_sectors(0x%x, 0x%x)", sector, count)

	segmentSector := sector - int64(seg.sectorOffset)
	r := make([][]byte, 0)

	tableIdx := sort.Search(len(seg.tableOffsets), func(i int) bool { return seg.tableOffsets[i] > segmentSector })
	for count > 0 {
		table := seg.tables[tableIdx]

		tableRemainingSectors := table.SectorCount - (segmentSector - table.SectorOffset)
		tableSectors := int64(math.Min(float64(tableRemainingSectors), float64(count)))

		data := table.readSectors(uint64(segmentSector), uint64(tableSectors))

		r = append(r, data)

		segmentSector += tableSectors
		count -= int(tableSectors)

		tableIdx++
		if tableIdx >= len(seg.tables) {
			break
		}
	}

	return bytes.Join(r, []byte{}), nil
}
