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
	NumEntries uint32 // header
	Pad        uint32 // header
	BaseOffset uint64 // header
	Pad2       uint32 // header
	Checksum   uint32 // header
}

type EWFTableSectionEntries struct {
	position int64
	Data     []uint32
}

type EWFTableSectionFooter struct {
	Checksum uint32
}

func (e *EWFTableSection) totalDataSize() int {
	return binary.Size(e.Header.NumEntries) +
		binary.Size(e.Header.Pad) +
		binary.Size(e.Header.BaseOffset) +
		binary.Size(e.Header.Pad2) +
		binary.Size(e.Header.Checksum) +
		binary.Size(e.Entries.Data) +
		binary.Size(e.Footer.Checksum)
}

func (e *EWFTableSection) serialize() (buf []byte, err error) {
	bbuf := bytes.NewBuffer(nil)

	_, e.Header.Checksum, err = WriteWithSum(bbuf, e.Header)
	if err != nil {
		return nil, err
	}

	dataLen := bbuf.Len()
	err = binary.Write(bbuf, binary.LittleEndian, e.Entries.Data)
	if err != nil {
		return
	}

	// only entries data
	e.Footer.Checksum = adler32.Checksum(bbuf.Bytes()[dataLen:])
	err = binary.Write(bbuf, binary.LittleEndian, e.Footer.Checksum)
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
	Entries      *EWFTableSectionEntries
	Footer       *EWFTableSectionFooter
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

	err := d.readData()
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

	tableSz := d.totalDataSize()
	desc.Size = uint64(tableSz) + DescriptorSize
	desc.Next = uint64(currentPosition) + desc.Size

	_, desc.Checksum, err = WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}

	d.Section = &EWFSectionDescriptor{
		Descriptor: desc,
	}

	headerData, err := d.serialize()
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

func (t *EWFTableSection) readData() error {
	if _, err := t.fh.Seek(t.Section.DataOffset, io.SeekStart); err != nil {
		return err
	}

	section := EWFTableSectionHeader{}

	err := binary.Read(t.fh, binary.LittleEndian, &section)
	if err != nil {
		return err
	}
	t.Header = &section

	entriesPosition, err := t.fh.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil
	}
	t.Entries = &EWFTableSectionEntries{
		position: entriesPosition,
	}

	return nil
}

func (t *EWFTableSection) getEntry(index int64) (entryPosition uint32, err error) {
	if t.Header.NumEntries > 0 && len(t.Entries.Data) > 0 {
		return t.Entries.Data[index], nil
	}

	cpos, err := t.fh.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	defer func() {
		// revert back to initial position
		_, errs := t.fh.Seek(cpos, io.SeekStart)
		if err == nil {
			err = errs
		}
	}()

	_, err = t.fh.Seek(t.Entries.position, io.SeekStart)
	if err != nil {
		return
	}

	t.Entries.Data = make([]uint32, t.Header.NumEntries)
	err = binary.Read(t.fh, binary.LittleEndian, &t.Entries.Data)
	if err != nil {
		return
	}
	entryPosition = t.Entries.Data[index]
	return
}

func (t *EWFTableSection) addEntry(offset uint32) {
	t.Header.NumEntries++
	//its always compressed
	e := offset | (1 << 31)
	t.Entries.Data = append(t.Entries.Data, e)
}

func (t *EWFTableSection) readChunk(chunk int64) ([]byte, error) {

	if chunk < 0 || chunk >= int64(t.Header.NumEntries) {
		return nil, errors.New("invalid chunk index")
	}

	chunkEntry, err := t.getEntry(chunk)
	if err != nil {
		return nil, err
	}
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
		che, err := t.getEntry(chunk + 1)
		if err != nil {
			return nil, err
		}
		chunkSize = t.BaseOffset + int64(che&0x7FFFFFFF) - int64(chunkOffset)
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
