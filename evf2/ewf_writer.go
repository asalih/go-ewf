package evf2

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/binary"
	"hash"
	"io"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/asalih/go-ewf/shared"
)

var _ shared.EWFWriter = &EWFWriter{}

type writer struct {
	position int64
	fh       io.Writer
}

func (w *writer) Write(p []byte) (n int, err error) {
	n, err = w.fh.Write(p)
	w.position += int64(n)
	return n, err
}

// EWFWriter is helper for creating Ex01 images. Data is always compressed
type EWFWriter struct {
	mu   sync.Mutex
	dest *writer

	dataPadSize int
	dataSize    uint64
	buf         []byte

	previousDescriptorPosition int64

	md5Hasher  hash.Hash
	sha1Hasher hash.Hash

	Segment       *EWFSegment
	SegmentOffset uint32
	ChunkSize     uint32
}

func CreateEWF(dest io.Writer) (*EWFWriter, error) {
	ewf := &EWFWriter{
		dest:          &writer{fh: dest},
		buf:           make([]byte, 0, DefaultChunkSize),
		SegmentOffset: 0,
		ChunkSize:     0,
	}

	var err error
	ewf.Segment, err = NewEWFSegment(nil)
	if err != nil {
		return nil, err
	}
	ewf.Segment.EWFHeader = &EWFHeader{
		MajorVersion:      2,
		MinorVersion:      1,
		SegmentNumber:     1, // TODO: this number increments for each file chunk like E0n
		CompressionMethod: EWF_COMPRESSION_METHOD_ZLIB,
	}
	copy(ewf.Segment.EWFHeader.Signature[:], []byte(EVF2Signature))

	ewf.Segment.CaseData = &EWFCaseDataSection{}
	ewf.Segment.CaseData.NumberOfObjects = "1"
	ewf.Segment.CaseData.ObjectName = "main"
	ewf.Segment.CaseData.KeyValue = make(map[string]string)

	ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	ewf.AddCaseData(EWF_CASE_DATA_CASE_NUMBER, "")
	ewf.AddCaseData(EWF_CASE_DATA_EXAMINER_NAME, "")
	ewf.AddCaseData(EWF_CASE_DATA_NOTES, "")
	ewf.AddCaseData(EWF_CASE_DATA_ACTUAL_TIME, ts)
	ewf.AddCaseData(EWF_CASE_DATA_EVIDENCE_NUMBER, "")
	ewf.AddCaseData(EWF_CASE_DATA_OS, runtime.GOOS)
	ewf.AddCaseData(EWF_CASE_DATA_NAME, "")
	ewf.AddCaseData(EWF_CASE_DATA_WRITE_BLOCKER_TYPE, "")
	ewf.AddCaseData(EWF_CASE_DATA_TARGET_TIME, ts)
	ewf.AddCaseData(EWF_CASE_DATA_COMPRESSION_METHOD, "1") //LZ
	ewf.AddCaseData(EWF_CASE_DATA_ERROR_GRANULARITY, "")

	ewf.Segment.DeviceInformation = &EWFDeviceInformationSection{}
	ewf.Segment.DeviceInformation.NumberOfObjects = "1"
	ewf.Segment.DeviceInformation.ObjectName = "main"
	ewf.Segment.DeviceInformation.KeyValue = make(map[string]string)

	ewf.AddDeviceInformation(EWF_DEVICE_INFO_DRIVE_MODEL, "")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_NUMBER_OF_DCO, "")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_PROCESS_ID, "")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_NUMBER_OF_SMART_LOGS, "")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_SERIAL_NUMBER, "")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_DRIVE_LABEL, "")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_NUMBER_OF_HPA, "")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_DRIVE_TYPE, "f")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_MUMBER_OF_PALM, "")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_IS_PHYSICAL, "1")

	ewf.Segment.Sectors = new(EWFSectorsSection)

	ewf.Segment.Tables = []*EWFTableSection{
		newTable(),
	}

	ewf.Segment.MD5Hash = new(EWFMD5Section)
	ewf.Segment.SHA1Hash = new(EWFSHA1Section)

	ewf.Segment.Done = new(EWFDoneSection)

	ewf.md5Hasher = md5.New()
	ewf.sha1Hasher = sha1.New()

	return ewf, nil
}

func (ewf *EWFWriter) AddCaseData(key EWFCaseDataInformationKey, value string) {
	ewf.Segment.CaseData.KeyValue[string(key)] = value
}

func (ewf *EWFWriter) AddDeviceInformation(key EWFDeviceInformationKey, value string) {
	ewf.Segment.DeviceInformation.KeyValue[string(key)] = value
}

func (ewf *EWFWriter) Start(totalSize int64) error {
	err := ewf.Segment.EWFHeader.Encode(ewf.dest)
	if err != nil {
		return err
	}

	headerPad, _ := alignSizeTo16Bytes(binary.Size(ewf.Segment.EWFHeader))
	_, err = ewf.dest.Write(headerPad)
	if err != nil {
		return err
	}

	numChunks := totalSize / DefaultChunkSize
	if totalSize%DefaultChunkSize > 0 {
		numChunks++
	}

	ewf.AddDeviceInformation(EWF_DEVICE_INFO_BYTES_PER_SEC, "512")
	ewf.AddDeviceInformation(EWF_DEVICE_INFO_NUMBER_OF_SECTORS, strconv.FormatInt(numChunks*64, 10))
	_, descN, err := ewf.Segment.DeviceInformation.Encode(ewf.dest, 0)
	if err != nil {
		return err
	}

	ewf.previousDescriptorPosition = ewf.dest.position - int64(descN)

	ewf.AddCaseData(EWF_CASE_DATA_NUMBER_OF_CHUNKS, strconv.FormatInt(numChunks, 10))
	ewf.AddCaseData(EWF_CASE_DATA_NUMBER_OF_SECTORS_PC, "64")
	ewf.AddCaseData(EWF_CASE_DATA_ERROR_GRANULARITY, "64")
	_, descN, err = ewf.Segment.CaseData.Encode(ewf.dest, ewf.previousDescriptorPosition)
	if err != nil {
		return err
	}
	ewf.previousDescriptorPosition = ewf.dest.position - int64(descN)

	return nil
}

func (ewf *EWFWriter) Write(p []byte) (n int, err error) {
	ewf.mu.Lock()
	defer ewf.mu.Unlock()

	ewf.buf = append(ewf.buf, p...)
	n = len(p)

	if len(ewf.buf) < DefaultChunkSize {
		return
	}

	for len(ewf.buf) >= DefaultChunkSize {
		_, err = ewf.writeData(ewf.buf[:DefaultChunkSize])
		if err != nil {
			return
		}

		ewf.buf = ewf.buf[DefaultChunkSize:]
	}

	return
}

func (ewf *EWFWriter) Close() error {
	if len(ewf.buf) > 0 {
		ewf.mu.Lock()
		ewf.buf = shared.PadBytes(ewf.buf, DefaultChunkSize)
		_, err := ewf.writeData(ewf.buf)
		if err != nil {
			ewf.mu.Unlock()
			return err
		}
		ewf.buf = ewf.buf[:0]
		ewf.mu.Unlock()
	}

	_, descN, err := ewf.Segment.Sectors.Encode(ewf.dest, ewf.dataSize, uint32(ewf.dataPadSize), ewf.previousDescriptorPosition)
	if err != nil {
		return err
	}

	ewf.previousDescriptorPosition = ewf.dest.position - int64(descN)

	for _, tbl := range ewf.Segment.Tables {
		_, descN, err := tbl.Encode(ewf.dest, ewf.previousDescriptorPosition)
		if err != nil {
			return err
		}
		ewf.previousDescriptorPosition = ewf.dest.position - int64(descN)
	}

	copy(ewf.Segment.MD5Hash.Hash[:], ewf.md5Hasher.Sum(nil))
	_, descN, err = ewf.Segment.MD5Hash.Encode(ewf.dest, ewf.previousDescriptorPosition)
	if err != nil {
		return err
	}
	ewf.previousDescriptorPosition = ewf.dest.position - int64(descN)

	copy(ewf.Segment.SHA1Hash.Hash[:], ewf.sha1Hasher.Sum(nil))
	_, descN, err = ewf.Segment.SHA1Hash.Encode(ewf.dest, ewf.previousDescriptorPosition)
	if err != nil {
		return err
	}
	ewf.previousDescriptorPosition = ewf.dest.position - int64(descN)

	_, descN, err = ewf.Segment.Done.Encode(ewf.dest, ewf.previousDescriptorPosition)
	if err != nil {
		return err
	}
	ewf.previousDescriptorPosition = ewf.dest.position - int64(descN)

	return nil
}

func (ewf *EWFWriter) writeData(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	var bufc []byte
	bufc, err = shared.CompressZlib(p)
	if err != nil {
		return
	}

	cpos := uint32(ewf.dest.position)
	n, err = ewf.dest.Write(bufc)
	ewf.dataSize += uint64(n)
	if err != nil {
		return
	}

	alignPad, padSize := alignSizeTo16Bytes(len(bufc))
	_, err = ewf.dest.Write(alignPad)
	if err != nil {
		return
	}
	ewf.dataPadSize += padSize

	ewf.Segment.addTableEntry(cpos, uint32(len(bufc)))

	_, err = ewf.md5Hasher.Write(p)
	if err != nil {
		return
	}

	_, err = ewf.sha1Hasher.Write(p)
	return
}
