package ewf

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
)

const (
	evfSig = "EVF\x09\x0d\x0a\xff\x00"
	lvfSig = "LVF\x09\x0d\x0a\xff\x00"
)

type EWFHeader struct {
	Signature     [8]byte
	FieldsStart   uint8
	SegmentNumber uint16
	FieldsEnd     uint16
}

func (e *EWFHeader) Decode(fh io.Reader) error {
	return binary.Read(fh, binary.LittleEndian, e)
}

func (e *EWFHeader) Encode(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, e)
}

type EWFSegment struct {
	ewfheader    *EWFHeader
	Header       *EWFHeaderSection
	Volume       *EWFVolumeSection
	sections     []*EWFSectionDescriptor
	Tables       []*EWFTableSection
	tableOffsets []int64
	chunkCount   int
	sectorCount  int
	sectorOffset int
	size         int
	offset       int64
}

func NewEWFSegment() *EWFSegment {
	return &EWFSegment{
		sections:     make([]*EWFSectionDescriptor, 0),
		Tables:       make([]*EWFTableSection, 0),
		tableOffsets: make([]int64, 0),
		chunkCount:   0,
		sectorCount:  0,
		sectorOffset: 0,
		size:         0,
		offset:       0,
	}
}

func (seg *EWFSegment) Decode(fh io.ReadSeeker) error {
	ewfHeader := new(EWFHeader)
	err := ewfHeader.Decode(fh)
	if err != nil {
		return err
	}
	sig := string(ewfHeader.Signature[:])
	if sig != evfSig && sig != lvfSig {
		return fmt.Errorf("invalid signature, got %v", ewfHeader.Signature)
	}
	seg.ewfheader = ewfHeader

	offset := int64(0)
	sectorOffset := int64(0)

	for {
		section, err := NewEWFSectionDescriptor(fh)
		if err != nil {
			return err
		}
		fmt.Println(section.Type)
		seg.sections = append(seg.sections, section)

		if section.Type == EWF_SECTION_TYPE_HEADER || section.Type == EWF_SECTION_TYPE_HEADER2 && seg.Header == nil {
			h := new(EWFHeaderSection)
			err := h.Decode(fh, section, seg)
			if err != nil {
				return err
			}
			seg.Header = h
		}

		if section.Type == EWF_SECTION_TYPE_DISK || section.Type == EWF_SECTION_TYPE_VOLUME && seg.Volume == nil {
			v := new(EWFVolumeSection)
			err := v.Decode(fh, section, seg)
			if err != nil {
				return err
			}
			seg.Volume = v
		}

		if section.Type == EWF_SECTION_TYPE_TABLE {
			table := new(EWFTableSection)
			err := table.Decode(fh, section, seg)
			if err != nil {
				return err
			}

			if sectorOffset != 0 {
				seg.tableOffsets = append(seg.tableOffsets, sectorOffset)
			}

			table.Offset = sectorOffset * int64(seg.Volume.Data.GetSectorSize())
			table.SectorOffset = sectorOffset
			sectorOffset += table.SectorCount

			seg.Tables = append(seg.Tables, table)
		}

		if section.Next == uint64(offset) || section.Type == EWF_SECTION_TYPE_DONE {
			break
		}

		offset = int64(section.Next)
		fh.Seek(offset, io.SeekStart)
	}

	for _, t := range seg.Tables {
		seg.chunkCount += int(t.Header.NumEntries)
	}

	seg.sectorCount = seg.chunkCount * int(seg.Volume.Data.GetSectorCount())
	seg.size = seg.chunkCount * int(seg.Volume.Data.GetSectorCount()) * int(seg.Volume.Data.GetSectorSize())

	return nil
}

func (seg *EWFSegment) ReadSectors(sector int64, count int) ([]byte, error) {
	segmentSector := sector - int64(seg.sectorOffset)
	buf := make([]byte, 0)

	tableIdx := sort.Search(len(seg.tableOffsets), func(i int) bool { return seg.tableOffsets[i] > segmentSector })
	for count > 0 {
		table := seg.Tables[tableIdx]

		tableRemainingSectors := table.SectorCount - (segmentSector - table.SectorOffset)
		tableSectors := int64(math.Min(float64(tableRemainingSectors), float64(count)))

		data := table.readSectors(uint64(segmentSector), uint64(tableSectors))

		buf = append(buf, data...)

		segmentSector += tableSectors
		count -= int(tableSectors)

		tableIdx++
		if tableIdx >= len(seg.Tables) {
			break
		}
	}

	return buf, nil
}