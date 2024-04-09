package ewf

import (
	"encoding/binary"
	"io"
)

type EWFDataSection struct {
	MediaType        uint8
	Unknown1         [3]uint8
	ChunkCount       uint32
	SectorPerChunk   uint32
	BytesPerSector   uint32
	Sectors          uint64
	CylindersCHS     uint32
	HeadesCHS        uint32
	SectorsCHS       uint32
	MediaFlags       uint8
	Uknown2          [3]uint8
	PALM             uint32
	Unkown3          [4]uint8
	SMART            uint32
	CompressionLevel uint8
	Unknown4         [3]uint8
	SectorErrorGr    [4]uint8
	Unknown5         [4]uint8
	GUID             [16]uint8
	Signature        [5]uint8
	Checksum         uint32
}

func (d *EWFDataSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, segment *EWFSegment) error {
	_, err := fh.Seek(section.DataOffset, io.SeekStart)
	if err != nil {
		return err
	}

	err = binary.Read(fh, binary.LittleEndian, d)
	if err != nil {
		return err
	}

	return nil
}

func (d *EWFDataSection) Encode(ewf io.WriteSeeker) error {
	currentPosition, err := ewf.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_DATA)

	dataSize := uint64(binary.Size(d))

	desc.Size = dataSize
	desc.Next = dataSize + DescriptorSize + uint64(currentPosition)

	_, desc.Checksum, err = WriteWithSum(ewf, desc)
	if err != nil {
		return err
	}

	_, d.Checksum, err = WriteWithSum(ewf, d)
	return err
}
