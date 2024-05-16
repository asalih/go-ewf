package evf2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/adler32"
	"io"
	"math"

	"github.com/asalih/go-ewf/shared"
)

type EWFTableSectionHeader struct {
	FirstChunkNumber uint64 // header
	NumEntries       uint32 // header
	Pad              uint32 // header
	Checksum         uint32 // header
}

type EWFTableSectionEntries struct {
	position int64
	Data     []EWFTableSectionEntry
}
type EWFTableSectionEntry struct {
	DataOffset uint64
	Size       uint32
	DataFlags  uint32
}

type EWFTableSectionFooter struct {
	Checksum uint32
}

func (e *EWFTableSection) serialize() (buf []byte, totalPaddingSize int, err error) {
	bbuf := bytes.NewBuffer(nil)

	err = binary.Write(bbuf, binary.LittleEndian, e.Header.FirstChunkNumber)
	if err != nil {
		return nil, 0, err
	}
	err = binary.Write(bbuf, binary.LittleEndian, e.Header.NumEntries)
	if err != nil {
		return nil, 0, err
	}
	err = binary.Write(bbuf, binary.LittleEndian, e.Header.Pad)
	if err != nil {
		return nil, 0, err
	}

	e.Header.Checksum = adler32.Checksum(bbuf.Bytes())
	err = binary.Write(bbuf, binary.LittleEndian, e.Header.Checksum)
	if err != nil {
		return nil, 0, err
	}

	headerPad, headerPaddingSize := alignSizeTo16Bytes(bbuf.Len())
	_, err = bbuf.Write(headerPad)
	if err != nil {
		return nil, 0, err
	}

	headerLen := bbuf.Len()
	err = binary.Write(bbuf, binary.LittleEndian, e.Entries.Data)
	if err != nil {
		return
	}

	// only entries data
	restLen := bbuf.Len()
	e.Footer.Checksum = adler32.Checksum(bbuf.Bytes()[headerLen:])
	err = binary.Write(bbuf, binary.LittleEndian, e.Footer)
	if err != nil {
		return
	}

	footerPadding, footerPaddingSize := alignSizeTo16Bytes(bbuf.Len() - restLen)
	_, err = bbuf.Write(footerPadding)
	if err != nil {
		return nil, 0, err
	}

	totalPaddingSize = headerPaddingSize + footerPaddingSize
	buf = bbuf.Bytes()
	return
}

type EWFTableSection struct {
	fh               io.ReadSeeker
	decompressorFunc shared.Decompressor

	Section      *EWFSectionDescriptor
	Segment      *EWFSegment
	Header       *EWFTableSectionHeader
	Entries      *EWFTableSectionEntries
	Footer       *EWFTableSectionFooter
	SectorCount  int64
	SectorOffset int64
	Size         int64
	Offset       int64
}

func newTable() *EWFTableSection {
	return &EWFTableSection{
		Header:  &EWFTableSectionHeader{},
		Entries: &EWFTableSectionEntries{},
		Footer:  &EWFTableSectionFooter{},
	}
}

func (d *EWFTableSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment, decompressorFunc shared.Decompressor) error {
	d.fh = fh
	d.Segment = segment
	d.Section = section
	d.decompressorFunc = decompressorFunc

	if _, err := d.fh.Seek(d.Section.DataOffset, io.SeekStart); err != nil {
		return err
	}

	err := d.readData()
	if err != nil {
		return err
	}

	sc, err := segment.CaseData.GetSectorCount()
	if err != nil {
		return err
	}
	ss, err := segment.DeviceInformation.GetSectorSize()
	if err != nil {
		return err
	}
	d.SectorCount = int64(d.Header.NumEntries) * int64(sc)
	d.SectorOffset = -1 // uninitialized
	d.Size = d.SectorCount * int64(ss)

	return nil
}

func (d *EWFTableSection) Encode(ewf io.Writer, previousDescriptorPosition int64) (dataN int, descN int, err error) {

	headerData, paddingSize, err := d.serialize()
	if err != nil {
		return 0, 0, err
	}
	dataN, err = ewf.Write(headerData)
	if err != nil {
		return 0, 0, nil
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_SECTOR_TABLE)

	desc.DataSize = uint64(dataN)
	desc.PreviousOffset = uint64(previousDescriptorPosition)
	desc.PaddingSize = uint32(paddingSize)

	descN, desc.Checksum, err = shared.WriteWithSum(ewf, desc)
	if err != nil {
		return 0, 0, err
	}

	d.Section = &EWFSectionDescriptor{
		Descriptor: desc,
	}

	return dataN, descN, nil
}

func (t *EWFTableSection) readData() error {
	if _, err := t.fh.Seek(t.Section.DataOffset, io.SeekStart); err != nil {
		return err
	}

	headerSection := EWFTableSectionHeader{}
	err := binary.Read(t.fh, binary.LittleEndian, &headerSection)
	if err != nil {
		return err
	}
	t.Header = &headerSection

	padSizeForHeader := calculatePadding(binary.Size(headerSection))
	cpos, err := t.fh.Seek(int64(padSizeForHeader), io.SeekCurrent)
	if err != nil {
		return err
	}

	t.Entries = &EWFTableSectionEntries{
		position: cpos,
	}

	return nil
}

func (t *EWFTableSection) getEntry(index int64) (entryPosition EWFTableSectionEntry, err error) {
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

	t.Entries.Data = make([]EWFTableSectionEntry, t.Header.NumEntries)
	err = binary.Read(t.fh, binary.LittleEndian, &t.Entries.Data)
	if err != nil {
		return
	}

	entryPosition = t.Entries.Data[index]
	return
}

func (t *EWFTableSection) readChunk(chunk int64) ([]byte, error) {
	if chunk < 0 || chunk >= int64(t.Header.NumEntries) {
		return nil, errors.New("invalid chunk index")
	}

	entry, err := t.getEntry(chunk)
	if err != nil {
		return nil, err
	}

	if _, err := t.fh.Seek(int64(entry.DataOffset), io.SeekStart); err != nil {
		return nil, err
	}

	buf := make([]byte, entry.Size)
	if _, err := io.ReadFull(t.fh, buf); err != nil {
		return nil, err
	}

	if entry.DataFlags&EWF_CHUNK_DATA_FLAG_USES_PATTERN_FILL != 0 { // PATTERNFILL
		sc, err := t.Segment.CaseData.GetSectorCount()
		if err != nil {
			return nil, err
		}
		ss, err := t.Segment.DeviceInformation.GetSectorSize()
		if err != nil {
			return nil, err
		}
		patternData, err := unpackFrom64BitPatternFill(buf, sc*ss)
		if err != nil {
			return nil, err
		}
		return patternData, nil
	}

	if entry.DataFlags&EWF_CHUNK_DATA_FLAG_IS_COMPRESSED != 0 { // COMPRESSED
		return t.decompressorFunc(buf)
	}
	if len(buf) <= ChecksumSize {
		return buf, nil
	}

	return buf[:len(buf)-ChecksumSize], nil
}

func unpackFrom64BitPatternFill(p []byte, chunkSize int) ([]byte, error) {
	if len(p) != 8 {
		return nil, errors.New("invalid compressed data size")
	}

	result := make([]byte, chunkSize)
	// Fill the uncompressed buffer with the pattern
	for i := 0; i < chunkSize; i += 8 {
		copy(result[i:], p)
	}

	return result, nil
}

func (ets *EWFTableSection) readSectors(sector uint64, count uint64) ([]byte, error) {
	if count == 0 {
		return nil, nil // Early return if there are no sectors to read
	}

	sectorSize, err := ets.Segment.DeviceInformation.GetSectorSize()
	if err != nil {
		return nil, err
	}

	chunkSectorCount, err := ets.Segment.CaseData.GetSectorCount()
	if err != nil {
		return nil, err
	}

	allBuf := make([]byte, 0, count*uint64(sectorSize))

	tableSector := sector - uint64(ets.SectorOffset)
	tableChunk := tableSector / uint64(chunkSectorCount)

	for count > 0 {
		tableSectorOffset := tableSector % uint64(chunkSectorCount)
		chunkRemainingSectors := uint64(chunkSectorCount) - tableSectorOffset
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
