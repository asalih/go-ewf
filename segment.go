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
	chunkCount   int
	sectorCount  int
	sectorOffset int
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
		if sig != evfSig && sig != lvfSig {
			return nil, fmt.Errorf("invalid signature, got %v", ewfHeader.Signature)
		}
		seg.EWFHeader = ewfHeader
	}

	return seg, nil
}

func (seg *EWFSegment) Decode(vol *EWFVolumeSection) error {

	offset := int64(0)
	sectorOffset := int64(0)

	if vol != nil {
		seg.Volume = vol
	}

	for {
		section, err := NewEWFSectionDescriptor(seg.fh)
		if err != nil {
			return err
		}
		seg.SectionDescriptors = append(seg.SectionDescriptors, section)

		if (section.Type == EWF_SECTION_TYPE_HEADER || section.Type == EWF_SECTION_TYPE_HEADER2) && seg.Header == nil {
			h := new(EWFHeaderSection)
			err := h.Decode(seg.fh, section, seg)
			if err != nil {
				return err
			}
			seg.Header = h
		}

		if section.Type == EWF_SECTION_TYPE_DISK || section.Type == EWF_SECTION_TYPE_VOLUME && seg.Volume == nil {
			v := new(EWFVolumeSection)
			err := v.Decode(seg.fh, section, seg)
			if err != nil {
				return err
			}
			seg.Volume = v
		}
		if section.Type == EWF_SECTION_TYPE_SECTORS {
			v := new(EWFSectorsSection)
			err := v.Decode(seg.fh, section, seg)
			if err != nil {
				return err
			}
			seg.Sectors = v
		}

		if section.Type == EWF_SECTION_TYPE_TABLE {
			table := new(EWFTableSection)
			err := table.Decode(seg.fh, section, seg)
			if err != nil {
				return err
			}

			if sectorOffset != 0 {
				seg.tableOffsets = append(seg.tableOffsets, sectorOffset)
			}

			table.SectorOffset = sectorOffset
			sectorOffset += table.SectorCount

			seg.Tables = append(seg.Tables, table)
		}

		if section.Type == EWF_SECTION_TYPE_DIGEST {
			dig := new(EWFDigestSection)
			err := dig.Decode(seg.fh, section, seg)
			if err != nil {
				return err
			}

			seg.Digest = dig
		}

		if section.Type == EWF_SECTION_TYPE_HASH {
			hashSec := new(EWFHashSection)
			err := hashSec.Decode(seg.fh, section, seg)
			if err != nil {
				return err
			}

			seg.Hash = hashSec
		}

		if section.Type == EWF_SECTION_TYPE_DATA {
			dataSec := new(EWFDataSection)
			err := dataSec.Decode(seg.fh, section, seg)
			if err != nil {
				return err
			}

			seg.Data = dataSec
		}

		if section.Type == EWF_SECTION_TYPE_DONE {
			doneSec := new(EWFDoneSection)
			err := doneSec.Decode(seg.fh, section, seg)
			if err != nil {
				return err
			}

			seg.Done = doneSec
		}

		if section.Next == uint64(offset) || section.Type == EWF_SECTION_TYPE_DONE {
			break
		}

		offset = int64(section.Next)
		seg.fh.Seek(offset, io.SeekStart)
	}

	for _, t := range seg.Tables {
		seg.chunkCount += int(t.Header.NumEntries)
	}

	seg.sectorCount = seg.chunkCount * int(seg.Volume.Data.GetSectorCount())

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
