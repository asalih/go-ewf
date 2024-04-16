package ewf

import (
	"encoding/binary"
	"io"
)

type EWFVolume interface {
	GetSectorSize() uint32
	GetSectorCount() uint32
	GetChunkCount() uint32
	IncrementChunkCount()
	GetChecksum() uint32
	SetChecksum(uint32)
}

type EWFVolumeSection struct {
	Data EWFVolume

	position int64
}

type EWFVolumeSectionSpecData struct {
	Reserved         uint32
	ChunkCount       uint32
	SectorCount      uint32
	SectorSize       uint32
	TotalSectorCount uint32
	Reserved1        [20]byte
	Pad              [45]byte
	Signature        [5]byte
	Checksum         uint32
}

func DefaultVolume() *EWFVolumeSectionData {
	return &EWFVolumeSectionData{
		MediaType:        Fixed,
		Reserved1:        [3]uint8{},
		SectorCount:      64,
		SectorSize:       512,
		MediaFlags:       Image,
		Unknown1:         [3]uint8{},
		CompressionLevel: None,
		Unknown3:         [3]uint8{},
		UUID:             [16]uint8{},
		Pad:              [963]uint8{},
		Signature:        [5]byte{},
	}
}

func (e *EWFVolumeSectionSpecData) GetSectorSize() uint32  { return e.SectorSize }
func (e *EWFVolumeSectionSpecData) GetSectorCount() uint32 { return e.SectorCount }
func (e *EWFVolumeSectionSpecData) GetChunkCount() uint32  { return e.ChunkCount }
func (e *EWFVolumeSectionSpecData) GetChecksum() uint32    { return e.Checksum }
func (e *EWFVolumeSectionSpecData) SetChecksum(c uint32)   { e.Checksum = c }
func (e *EWFVolumeSectionSpecData) IncrementChunkCount() {
	e.ChunkCount++
	e.TotalSectorCount = e.ChunkCount * e.SectorCount
}

type EWFVolumeSectionData struct {
	MediaType        MediaType
	Reserved1        [3]byte
	ChunkCount       uint32
	SectorCount      uint32
	SectorSize       uint32
	TotalSectorCount uint64
	NumCylinders     uint32
	NumHeads         uint32
	NumSectors       uint32
	MediaFlags       MediaFlags
	Unknown1         [3]byte
	PalmStartSector  uint32
	Unknown2         uint32
	SmartStartSector uint32
	CompressionLevel CompressionLevel
	Unknown3         [3]byte
	ErrorGranularity uint32
	Unknown4         uint32
	UUID             [16]byte
	Pad              [963]byte
	Signature        [5]byte
	Checksum         uint32
}

func (e *EWFVolumeSectionData) GetSectorSize() uint32  { return e.SectorSize }
func (e *EWFVolumeSectionData) GetSectorCount() uint32 { return e.SectorCount }
func (e *EWFVolumeSectionData) GetChunkCount() uint32  { return e.ChunkCount }
func (e *EWFVolumeSectionData) GetChecksum() uint32    { return e.Checksum }
func (e *EWFVolumeSectionData) SetChecksum(c uint32)   { e.Checksum = c }
func (e *EWFVolumeSectionData) IncrementChunkCount() {
	e.ChunkCount++
	e.TotalSectorCount = uint64(e.ChunkCount) * uint64(e.GetSectorCount())
}

func (v *EWFVolumeSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) error {
	_, err := fh.Seek(section.DataOffset, io.SeekStart)
	if err != nil {
		return err
	}

	if section.Size == 0x41C {
		var dataSect EWFVolumeSectionData
		err := binary.Read(fh, binary.LittleEndian, &dataSect)
		if err != nil {
			return err
		}

		v.Data = &dataSect

		return nil
	}

	var dataSectSpec EWFVolumeSectionSpecData
	err = binary.Read(fh, binary.LittleEndian, &dataSectSpec)
	if err != nil {
		return err
	}

	v.Data = &dataSectSpec

	return nil
}

func (vol *EWFVolumeSection) Encode(ewf io.WriteSeeker) error {
	currentPosition, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	if vol.position <= 0 {
		desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_VOLUME)

		dataSize := uint64(binary.Size(vol.Data))

		desc.Size = dataSize + DescriptorSize
		desc.Next = desc.Size + uint64(currentPosition)

		var err error
		_, desc.Checksum, err = WriteWithSum(ewf, desc)
		if err != nil {
			return err
		}

		// the first write will be placeholder
		vol.position = currentPosition + int64(DescriptorSize)
		currentPosition = vol.position + int64(dataSize)
	}

	_, err = ewf.Seek(vol.position, io.SeekStart)
	if err != nil {
		return err
	}

	_, sum, err := WriteWithSum(ewf, vol.Data)
	if err != nil {
		return err
	}
	vol.Data.SetChecksum(sum)

	_, err = ewf.Seek(currentPosition, io.SeekStart)
	return err
}
