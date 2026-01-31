package evf1

import (
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

func (e *EWFHeader) Decode(fh io.Reader) error {
	return binary.Read(fh, binary.LittleEndian, e)
}

func (e *EWFHeader) Encode(ewf io.WriteSeeker) error {
	return binary.Write(ewf, binary.LittleEndian, e)
}

type EWFSegment struct {
	EWFHeader *EWFHeader
	Header    *EWFHeaderSection
	Volume    *EWFVolumeSection
	Sectors   *EWFSectorsSection
	Tables    []*EWFTableSection
	Digest    *EWFDigestSection
	Hash      *EWFHashSection
	Data      *EWFDataSection
	Done      *EWFDoneSection

	SectionDescriptors []*EWFSectionDescriptor

	fh           io.ReadSeeker
	isDecoded    bool
	chunkCount   int64
	sectorCount  int64
	sectorOffset int64
	tableOffsets []int64
}

func NewEWFSegment(fh io.ReadSeeker) (*EWFSegment, error) {
	seg := &EWFSegment{
		SectionDescriptors: make([]*EWFSectionDescriptor, 0),
		tableOffsets:       make([]int64, 0),
		fh:                 fh,
	}

	if fh != nil {
		ewfHeader := new(EWFHeader)
		err := ewfHeader.Decode(fh)
		if err != nil {
			return nil, err
		}
		sig := string(ewfHeader.Signature[:])
		if sig != EVFSignature && sig != LVFSignature {
			return nil, fmt.Errorf("invalid signature, got %v", ewfHeader.Signature)
		}
		seg.EWFHeader = ewfHeader
	}

	return seg, nil
}

func (seg *EWFSegment) Decode(link *EWFSegment) error {
	if seg.isDecoded {
		return nil
	}

	offset := int64(0)
	sectorOffset := int64(0)

	if link != nil && link.Volume != nil {
		seg.Volume = link.Volume
	}

	for {
		section, err := NewEWFSectionDescriptor(seg.fh)
		if err != nil {
			return err
		}
		seg.SectionDescriptors = append(seg.SectionDescriptors, section)

		switch section.Type {
		case EWF_SECTION_TYPE_HEADER, EWF_SECTION_TYPE_HEADER2:
			if seg.Header == nil {
				h := new(EWFHeaderSection)
				if err := h.Decode(seg.fh, section); err != nil {
					return err
				}
				seg.Header = h
			}

		case EWF_SECTION_TYPE_DISK, EWF_SECTION_TYPE_VOLUME:
			if seg.Volume == nil {
				v := new(EWFVolumeSection)
				if err := v.Decode(seg.fh, section); err != nil {
					return err
				}
				seg.Volume = v
			}

		case EWF_SECTION_TYPE_SECTORS:
			v := new(EWFSectorsSection)
			if err := v.Decode(seg.fh, section); err != nil {
				return err
			}
			seg.Sectors = v

		case EWF_SECTION_TYPE_TABLE:
			table := new(EWFTableSection)
			if err := table.Decode(seg.fh, section, seg); err != nil {
				return err
			}

			if sectorOffset != 0 {
				seg.tableOffsets = append(seg.tableOffsets, sectorOffset)
			}

			table.SectorOffset = sectorOffset
			sectorOffset += table.SectorCount

			seg.Tables = append(seg.Tables, table)

		case EWF_SECTION_TYPE_DIGEST:
			dig := new(EWFDigestSection)
			if err := dig.Decode(seg.fh, section); err != nil {
				return err
			}
			seg.Digest = dig

		case EWF_SECTION_TYPE_HASH:
			hashSec := new(EWFHashSection)
			if err := hashSec.Decode(seg.fh, section); err != nil {
				return err
			}
			seg.Hash = hashSec

		case EWF_SECTION_TYPE_DATA:
			dataSec := new(EWFDataSection)
			if err := dataSec.Decode(seg.fh, section); err != nil {
				return err
			}
			seg.Data = dataSec

		case EWF_SECTION_TYPE_DONE:
			doneSec := new(EWFDoneSection)
			if err := doneSec.Decode(seg.fh, section); err != nil {
				return err
			}
			seg.Done = doneSec

		default:
			// Handle any unknown section types or add a fallback here if needed
		}

		// Exit the loop if we have reached the end or a specific condition
		if section.Next == uint64(offset) || section.Type == EWF_SECTION_TYPE_DONE {
			break
		}

		// Update the offset and seek to the next section
		offset = int64(section.Next)
		if _, err := seg.fh.Seek(offset, io.SeekStart); err != nil {
			return err
		}
	}

	for _, t := range seg.Tables {
		seg.chunkCount += int64(t.Header.NumEntries)
	}

	seg.sectorCount = seg.chunkCount * int64(seg.Volume.Data.GetSectorCount())
	if link != nil {
		seg.sectorOffset = link.sectorOffset + link.sectorCount
	}
	seg.isDecoded = true

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

		data, err := table.readSectors(uint64(segmentSector), uint64(tableSectors))
		if err != nil {
			return buf, err
		}

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

// addTableEntry records a chunk at absolute file offset `absoluteOffset`.
//
// EVF1 table entries only have 31 bits for the offset (MSB is compression flag),
// so we must use the table header `BaseOffset` and store a 31-bit relative offset.
func (seg *EWFSegment) addTableEntry(absoluteOffset int64) error {
	if absoluteOffset < 0 {
		return fmt.Errorf("invalid negative chunk offset: %d", absoluteOffset)
	}

	t := seg.Tables[len(seg.Tables)-1]

	// If table is full, start a new one.
	if t.Header.NumEntries >= maxTableLength {
		t = newTable()
		seg.Tables = append(seg.Tables, t)
	}

	// Initialize BaseOffset at the first entry of the table.
	if t.Header.NumEntries == 0 {
		t.Header.BaseOffset = uint64(absoluteOffset)
	}

	base := t.Header.BaseOffset
	rel := uint64(absoluteOffset) - base

	// Entry offset is 31-bit (MSB reserved for flags). If relative offset overflows,
	// start a new table with a new BaseOffset.
	if rel > 0x7FFFFFFF {
		t = newTable()
		t.Header.BaseOffset = uint64(absoluteOffset)
		seg.Tables = append(seg.Tables, t)
		base = t.Header.BaseOffset
		rel = uint64(absoluteOffset) - base // should be 0
	}

	t.Header.NumEntries++
	// data is always written as zlib-compressed in this writer
	e := uint32(rel) | (1 << 31)
	t.Entries.Data = append(t.Entries.Data, e)

	return nil
}
