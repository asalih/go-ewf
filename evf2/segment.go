package evf2

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/asalih/go-ewf/shared"
)

type EWFHeader struct {
	Signature         [8]byte
	MajorVersion      uint8
	MinorVersion      uint8
	CompressionMethod uint16
	SegmentNumber     uint16
	SetIdentifier     [8]byte
}

func (e *EWFHeader) Decode(fh io.Reader) error {
	return binary.Read(fh, binary.LittleEndian, e)
}

func (e *EWFHeader) Encode(ewf io.Writer) error {
	return binary.Write(ewf, binary.LittleEndian, e)
}

type EWFSegment struct {
	EWFHeader         *EWFHeader
	DeviceInformation *EWFDeviceInformationSection
	CaseData          *EWFCaseDataSection

	Sectors  *EWFSectorsSection
	Tables   []*EWFTableSection
	MD5Hash  *EWFMD5Section
	SHA1Hash *EWFSHA1Section
	Done     *EWFDoneSection

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
		if sig != EVF2Signature && sig != LVF2Signature {
			return nil, fmt.Errorf("invalid signature, got %v", ewfHeader.Signature)
		}
		seg.EWFHeader = ewfHeader
	}

	return seg, nil
}

func (seg *EWFSegment) Decode(link *EWFSegment, decompressorFunc shared.Decompressor) error {
	if seg.isDecoded {
		return nil
	}

	// Assuming fh is positioned at the end of the file or where the last section ends
	var err error
	var section *EWFSectionDescriptor
	_, err = seg.fh.Seek(-DescriptorSize, io.SeekEnd)
	if err != nil {
		return err
	}

	offset, err := seg.fh.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	for offset > 0 {
		section, err = NewEWFSectionDescriptor(seg.fh)
		if err != nil {
			return err
		}

		// Append section descriptor in reverse order
		seg.SectionDescriptors = append([]*EWFSectionDescriptor{section}, seg.SectionDescriptors...)

		// Move to the previous section pointed by this section descriptor
		offset = int64(section.Previous)
		if offset == 0 {
			break
		}
		_, err := seg.fh.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}
	}

	// sections must be readed end to start direction
	// after reading descriptors, we decode the data
	sectorOffset := int64(0)
	for _, section := range seg.SectionDescriptors {
		// Process specific section types
		switch section.Type {
		case EWF_SECTION_TYPE_DEVICE_INFORMATION:
			if seg.DeviceInformation != nil {
				continue
			}
			h := new(EWFDeviceInformationSection)
			if err := h.Decode(seg.fh, section, decompressorFunc); err != nil {
				return err
			}
			seg.DeviceInformation = h

		case EWF_SECTION_TYPE_CASE_DATA:
			if seg.CaseData != nil {
				continue
			}
			h := new(EWFCaseDataSection)
			if err := h.Decode(seg.fh, section, decompressorFunc); err != nil {
				return err
			}
			seg.CaseData = h

		case EWF_SECTION_TYPE_SECTOR_DATA:
			sectorData := new(EWFSectorsSection)
			if err := sectorData.Decode(seg.fh, section); err != nil {
				return err
			}
			seg.Sectors = sectorData

		case EWF_SECTION_TYPE_SECTOR_TABLE:
			table := new(EWFTableSection)
			if err := table.Decode(seg.fh, section, seg, decompressorFunc); err != nil {
				return err
			}

			if sectorOffset != 0 {
				seg.tableOffsets = append(seg.tableOffsets, sectorOffset)
			}

			table.SectorOffset = sectorOffset
			sectorOffset += table.SectorCount

			seg.Tables = append(seg.Tables, table)
		case EWF_SECTION_TYPE_MD5_HASH:
			md5Hash := new(EWFMD5Section)
			if err := md5Hash.Decode(seg.fh, section); err != nil {
				return err
			}
			seg.MD5Hash = md5Hash
		case EWF_SECTION_TYPE_SHA1_HASH:
			sha1Hash := new(EWFSHA1Section)
			if err := sha1Hash.Decode(seg.fh, section); err != nil {
				return err
			}
			seg.SHA1Hash = sha1Hash
		case EWF_SECTION_TYPE_DONE:
			doneSec := new(EWFDoneSection)
			if err := doneSec.Decode(seg.fh, section, seg); err != nil {
				return err
			}
			seg.Done = doneSec
		}
	}

	for _, t := range seg.Tables {
		seg.chunkCount += int64(t.Header.NumEntries)
	}

	sc, err := seg.CaseData.GetSectorCount()
	if err != nil {
		return err
	}
	seg.sectorCount = seg.chunkCount * int64(sc)
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

// IMPORTANT
func (seg *EWFSegment) addTableEntry(offset int64, size uint32, flag uint32) {
	t := seg.Tables[len(seg.Tables)-1]

	t.Header.NumEntries++

	e := EWFTableSectionEntry{
		DataOffset: uint64(offset),
		Size:       size,
		DataFlags:  flag,
	}
	t.Entries.Data = append(t.Entries.Data, e)
}
