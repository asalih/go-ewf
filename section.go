package ewf

import (
	"encoding/binary"
	"io"
)

type EWFVolumeSection struct {
	Volume      interface{}
	SectorSize  uint32
	SectorCount uint32
	ChunkCount  uint32
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

func NewEWFVolumeSection(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) (*EWFVolumeSection, error) {
	fh.Seek(section.DataOffset, io.SeekStart)

	var dataSect EWFVolumeSectionData
	var dataSectSpec EWFVolumeSectionSpecData
	var err error

	if section.Size == 0x41C {
		err = binary.Read(fh, binary.LittleEndian, &dataSect)
		if err != nil {
			return nil, err
		}

		return &EWFVolumeSection{
			SectorSize:  dataSect.SectorSize,
			SectorCount: dataSect.SectorCount,
			ChunkCount:  dataSect.ChunkCount,
			Volume:      dataSect,
		}, nil
	}

	err = binary.Read(fh, binary.LittleEndian, &dataSectSpec)

	if err != nil {
		return nil, err
	}

	return &EWFVolumeSection{
		SectorSize:  dataSectSpec.SectorSize,
		SectorCount: dataSectSpec.SectorCount,
		ChunkCount:  dataSectSpec.ChunkCount,
		Volume:      dataSectSpec,
	}, nil
}
