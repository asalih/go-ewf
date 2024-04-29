package ewf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/adler32"
	"io"
	"math"
)

type EWFTableSectionHeader struct {
	NumEntries     uint32 // header
	Pad            uint32 // header
	BaseOffset     uint64 // header
	Pad2           uint32 // header
	Checksum       uint32 // header
	Entries        []uint32
	FooterChecksum uint32 // footer
}

func (e *EWFTableSectionHeader) size() int {
	return binary.Size(e.NumEntries) +
		binary.Size(e.Pad) +
		binary.Size(e.BaseOffset) +
		binary.Size(e.Pad2) +
		binary.Size(e.Checksum) +
		binary.Size(e.Entries) +
		binary.Size(e.FooterChecksum)
}

func (e *EWFTableSectionHeader) serialize() (buf []byte, err error) {
	bbuf := bytes.NewBuffer(nil)
	err = binary.Write(bbuf, binary.LittleEndian, e.NumEntries)
	if err != nil {
		return
	}

	err = binary.Write(bbuf, binary.LittleEndian, e.Pad)
	if err != nil {
		return
	}
	err = binary.Write(bbuf, binary.LittleEndian, e.BaseOffset)
	if err != nil {
		return
	}
	err = binary.Write(bbuf, binary.LittleEndian, e.Pad2)
	if err != nil {
		return
	}

	e.Checksum = adler32.Checksum(bbuf.Bytes())
	err = binary.Write(bbuf, binary.LittleEndian, e.Checksum)
	if err != nil {
		return
	}

	dataLen := bbuf.Len()
	err = binary.Write(bbuf, binary.LittleEndian, e.Entries)
	if err != nil {
		return
	}

	// only entries data
	e.FooterChecksum = adler32.Checksum(bbuf.Bytes()[dataLen:])
	err = binary.Write(bbuf, binary.LittleEndian, e.FooterChecksum)
	if err != nil {
		return
	}

	buf = bbuf.Bytes()
	return
}

type EWFTableSection struct {
	fh io.ReadSeeker

	Section      *EWFSectionDescriptor
	Segment      *EWFSegment
	Header       *EWFTableSectionHeader
	BaseOffset   int64
	SectorCount  int64
	SectorOffset int64
	Size         int64
	Offset       int64
}

func (d *EWFTableSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) error {

	d.fh = fh
	d.Segment = segment
	d.Section = section

	if _, err := d.fh.Seek(d.Section.DataOffset, io.SeekStart); err != nil {
		return err
	}

	err := d.readHeader()
	if err != nil {
		return err
	}

	d.BaseOffset = int64(d.Header.BaseOffset)

	d.SectorCount = int64(d.Header.NumEntries) * int64(segment.Volume.Data.GetSectorCount())
	d.SectorOffset = -1 // uninitialized
	d.Size = d.SectorCount * int64(segment.Volume.Data.GetSectorSize())

	return nil
}

func (d *EWFTableSection) Encode(ewf io.WriteSeeker) error {
	currentPosition, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_TABLE)

	tableSz := d.Header.size()
	desc.Size = uint64(tableSz) + DescriptorSize
	desc.Next = uint64(currentPosition) + desc.Size

	_, desc.Checksum, err = WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}

	d.Section = &EWFSectionDescriptor{
		Descriptor: desc,
	}

	headerData, err := d.Header.serialize()
	if err != nil {
		return err
	}

	_, err = ewf.Write(headerData)
	if err != nil {
		return err
	}

	currentPosition, err = ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	desc = NewEWFSectionDescriptorData(EWF_SECTION_TYPE_TABLE2)
	desc.Size = uint64(tableSz) + DescriptorSize
	desc.Next = uint64(currentPosition) + desc.Size

	_, _, err = WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}

	_, err = ewf.Write(headerData)
	return err
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

	err = binary.Read(t.fh, binary.LittleEndian, &section.FooterChecksum)
	if err != nil {
		return err
	}

	t.Header = &section

	return nil
}

func (t *EWFTableSection) addEntry(offset uint32) {
	t.Header.NumEntries++
	//its always compressed
	e := offset | (1 << 31)
	t.Header.Entries = append(t.Header.Entries, e)
}

func (t *EWFTableSection) readChunk(chunk int64) ([]byte, error) {

	if chunk < 0 || chunk >= int64(len(t.Header.Entries)) {
		return nil, errors.New("invalid chunk index")
	}

	chunkEntry := t.Header.Entries[chunk]
	chunkOffset := uint32(t.BaseOffset) + (chunkEntry & 0x7FFFFFFF)
	compressed := chunkEntry>>31 == 1

	var chunkSize int64
	if chunk+1 == int64(t.Header.NumEntries) {
		// The chunk data is stored before the table section
		chunkSize = t.calculateLastChunkSize(chunkOffset)
		if chunkSize == -1 {
			return nil, errors.New("unknown size of last chunk")
		}

	} else {
		chunkSize = t.BaseOffset + int64(t.Header.Entries[chunk+1]&0x7FFFFFFF) - int64(chunkOffset)
	}

	// Non compressed chunks have a 4 byte checksum
	if !compressed {
		chunkSize -= ChecksumSize
	}

	if _, err := t.fh.Seek(int64(chunkOffset), io.SeekStart); err != nil {
		return nil, err
	}

	buf := make([]byte, chunkSize)
	if _, err := io.ReadFull(t.fh, buf); err != nil {
		return nil, err
	}

	if compressed {
		return decompress(buf)
	}

	return buf, nil
}

// Helper function to calculate the size of the last chunk
func (t *EWFTableSection) calculateLastChunkSize(chunkOffset uint32) int64 {
	// EWF sucks
	// We don't know the chunk size, so try to determine it using the offset of the next chunk
	// When it's the last chunk in the table though, this becomes trickier.
	// We have to check if the chunk data is preceding the table, or if it's contained within the table section
	// Then we can calculate the chunk size using these offsets

	if chunkOffset < uint32(t.Section.offset) {
		return t.Section.offset - int64(chunkOffset)
	}

	if int64(chunkOffset) < t.Section.offset+int64(t.Section.Size) {
		return t.Section.offset + int64(t.Section.Size) - int64(chunkOffset)
	}

	return -1
}

func (ets *EWFTableSection) readSectors(sector uint64, count uint64) ([]byte, error) {
	if count == 0 {
		return nil, nil // Early return if there are no sectors to read
	}

	sectorSize := ets.Segment.Volume.Data.GetSectorSize()
	chunkSectorCount := ets.Segment.Volume.Data.GetSectorCount()

	allBuf := make([]byte, 0, count*uint64(sectorSize))

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
			return buf, err
		}
		if chunkPos != 0 || tableSectors != uint64(chunkSectorCount) {
			buf = buf[chunkPos:chunkEnd]
		}
		allBuf = append(allBuf, buf...)

		count -= tableSectors
		tableSector += tableSectors
		tableChunk += 1
	}

	return allBuf, nil
}
