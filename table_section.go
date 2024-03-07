package ewf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

type EWFTableSectionHeader struct {
	NumEntries uint32
	Pad        uint32
	BaseOffset uint64
	Pad2       uint32
	Checksum   uint32
	Entries    []uint32
}

type EWFTableSection struct {
	fh           io.ReadSeeker
	Section      *EWFSectionDescriptor
	Segment      *EWFSegment
	Header       *EWFTableSectionHeader
	BaseOffset   int64
	SectorCount  int64
	SectorOffset int64
	Size         int64
	Offset       int64
}

func NewEWFTableSection(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) (*EWFTableSection, error) {
	t := &EWFTableSection{
		fh:      fh,
		Section: section,
		Segment: segment,
	}

	if _, err := t.fh.Seek(t.Section.DataOffset, io.SeekStart); err != nil {
		return nil, err
	}

	err := t.readHeader()
	if err != nil {
		return nil, err
	}

	t.BaseOffset = int64(t.Header.BaseOffset)

	t.SectorCount = int64(t.Header.NumEntries) * int64(segment.volume.SectorCount)
	t.SectorOffset = -1 // uninitialized
	t.Size = t.SectorCount * int64(segment.volume.SectorSize)

	return t, nil
}

func (t *EWFTableSection) readHeader() error {
	if _, err := t.fh.Seek(t.Section.DataOffset, io.SeekStart); err != nil {
		return err
	}

	section := EWFTableSectionHeader{}

	err := binary.Read(t.fh, binary.LittleEndian, &section.NumEntries)
	if err != nil {
		return err
	}

	err = binary.Read(t.fh, binary.LittleEndian, &section.Pad)
	if err != nil {
		return err
	}

	err = binary.Read(t.fh, binary.LittleEndian, &section.BaseOffset)
	if err != nil {
		return err
	}

	err = binary.Read(t.fh, binary.LittleEndian, &section.Pad2)
	if err != nil {
		return err
	}

	err = binary.Read(t.fh, binary.LittleEndian, &section.Checksum)
	if err != nil {
		return err
	}

	section.Entries = make([]uint32, section.NumEntries)
	err = binary.Read(t.fh, binary.LittleEndian, &section.Entries)
	if err != nil {
		return err
	}

	t.Header = &section

	return nil
}

func (t *EWFTableSection) readChunk(chunk int64) ([]byte, error) {
	// log.Printf("EWFTableSection::read_chunk(0x%x)", chunk)

	if chunk < 0 || chunk >= int64(len(t.Header.Entries)) {
		return nil, errors.New("invalid chunk index")
	}

	chunkEntry := t.Header.Entries[chunk]
	chunkOffset := uint32(t.BaseOffset) + (chunkEntry & 0x7FFFFFFF)
	compressed := chunkEntry>>31 == 1

	// EWF sucks
	// We don't know the chunk size, so try to determine it using the offset of the next chunk
	// When it's the last chunk in the table though, this becomes trickier.
	// We have to check if the chunk data is preceding the table, or if it's contained within the table section
	// Then we can calculate the chunk size using these offsets
	var chunkSize int64
	if chunk+1 == int64(t.Header.NumEntries) {
		// The chunk data is stored before the table section
		if chunkOffset < uint32(t.Section.offset) {
			chunkSize = t.Section.offset - int64(chunkOffset)
		} else if int64(chunkOffset) < t.Section.offset+int64(t.Section.Size) {
			chunkSize = t.Section.offset + int64(t.Section.Size) - int64(chunkOffset)
		} else {
			return nil, errors.New("unknown size of last chunk")
		}
	} else {
		chunkSize = t.BaseOffset + int64(t.Header.Entries[chunk+1]&0x7FFFFFFF) - int64(chunkOffset)
	}

	// Non compressed chunks have a 4 byte checksum
	if !compressed {
		chunkSize -= 4
	}

	if _, err := t.fh.Seek(int64(chunkOffset), io.SeekStart); err != nil {
		return nil, err
	}

	buf := make([]byte, chunkSize)
	if _, err := io.ReadFull(t.fh, buf); err != nil {
		return nil, err
	}

	if compressed {
		reader, err := zlib.NewReader(bytes.NewReader(buf))
		if err != nil {
			return nil, err
		}

		return io.ReadAll(reader)
	}

	return buf, nil
}

func (ets *EWFTableSection) readSectors(sector uint64, count uint64) []byte {
	// log.Debug("EWFTableSection::read_sectors(0x%x, 0x%x)", sector, count)
	var r [][]byte

	chunkSectorCount := ets.Segment.ewf.Volume.SectorCount
	sectorSize := ets.Segment.ewf.Volume.SectorSize

	tableSector := sector - uint64(ets.SectorOffset)
	tableChunk := tableSector / uint64(chunkSectorCount)

	for count > 0 {
		tableSectorOffset := tableSector % uint64(chunkSectorCount)
		chunkRemainingSectors := chunkSectorCount - uint32(tableSectorOffset)
		tableSectors := uint64(math.Min(float64(chunkRemainingSectors), float64(count)))

		chunkPos := tableSectorOffset * uint64(sectorSize)
		chunkEnd := chunkPos + (tableSectors * uint64(sectorSize))

		buf, err := ets.readChunk(int64(tableChunk))
		if err != nil {
			//TODO: return err
			continue
		}
		if chunkPos != 0 || tableSectors != uint64(chunkSectorCount) {
			buf = buf[chunkPos:chunkEnd]
		}
		r = append(r, buf)

		count -= tableSectors
		tableSector += tableSectors
		tableChunk += 1
	}

	return bytes.Join(r, []byte{})
}
