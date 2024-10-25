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
	compressor  *shared.ZlibCompressor

	previousDescriptorPosition int64

	md5Hasher  hash.Hash
	sha1Hasher hash.Hash

	Segment       *EWFSegment
	SegmentOffset uint32
	ChunkSize     uint32
}

type EWFCreator struct {
	ewfWriter *EWFWriter
}

func CreateEWF(dest io.Writer) (*EWFCreator, error) {
	ewf := &EWFWriter{
		dest:          &writer{fh: dest},
		buf:           make([]byte, 0, DefaultChunkSize),
		SegmentOffset: 0,
		ChunkSize:     0,
	}

	compressor, err := shared.NewZlibCompressor()
	if err != nil {
		return nil, err
	}
	ewf.compressor = compressor

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
	ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	ewf.Segment.CaseData.KeyValue = map[string]string{
		string(EWF_CASE_DATA_CASE_NUMBER):        "",
		string(EWF_CASE_DATA_EXAMINER_NAME):      "",
		string(EWF_CASE_DATA_NOTES):              "",
		string(EWF_CASE_DATA_ACTUAL_TIME):        ts,
		string(EWF_CASE_DATA_EVIDENCE_NUMBER):    "",
		string(EWF_CASE_DATA_OS):                 runtime.GOOS,
		string(EWF_CASE_DATA_NAME):               "",
		string(EWF_CASE_DATA_WRITE_BLOCKER_TYPE): "",
		string(EWF_CASE_DATA_TARGET_TIME):        ts,
		string(EWF_CASE_DATA_COMPRESSION_METHOD): "1", //LZ
		string(EWF_CASE_DATA_ERROR_GRANULARITY):  "",
	}

	ewf.Segment.DeviceInformation = &EWFDeviceInformationSection{}
	ewf.Segment.DeviceInformation.NumberOfObjects = "1"
	ewf.Segment.DeviceInformation.ObjectName = "main"
	ewf.Segment.DeviceInformation.KeyValue = map[string]string{
		string(EWF_DEVICE_INFO_DRIVE_MODEL):          "",
		string(EWF_DEVICE_INFO_NUMBER_OF_SMART_LOGS): "",
		string(EWF_DEVICE_INFO_SERIAL_NUMBER):        "",
		string(EWF_DEVICE_INFO_DRIVE_LABEL):          "",
		string(EWF_DEVICE_INFO_NUMBER_OF_HPA):        "",
		string(EWF_DEVICE_INFO_DRIVE_TYPE):           "f",
		string(EWF_DEVICE_INFO_MUMBER_OF_PALM):       "",
		string(EWF_DEVICE_INFO_IS_PHYSICAL):          "1",
	}

	ewf.Segment.Sectors = new(EWFSectorsSection)
	ewf.Segment.Tables = []*EWFTableSection{
		newTable(),
	}

	ewf.Segment.MD5Hash = new(EWFMD5Section)
	ewf.Segment.SHA1Hash = new(EWFSHA1Section)

	ewf.Segment.Done = new(EWFDoneSection)

	ewf.md5Hasher = md5.New()
	ewf.sha1Hasher = sha1.New()

	return &EWFCreator{ewfWriter: ewf}, nil
}

func (creator *EWFCreator) AddCaseData(key EWFCaseDataInformationKey, value string) {
	creator.ewfWriter.Segment.CaseData.KeyValue[string(key)] = value
}

func (creator *EWFCreator) AddDeviceInformation(key EWFDeviceInformationKey, value string) {
	creator.ewfWriter.Segment.DeviceInformation.KeyValue[string(key)] = value
}

func (creator *EWFCreator) Start(totalSize int64) (*EWFWriter, error) {
	err := creator.ewfWriter.Segment.EWFHeader.Encode(creator.ewfWriter.dest)
	if err != nil {
		return nil, err
	}

	headerPad, _ := alignSizeTo16Bytes(binary.Size(creator.ewfWriter.Segment.EWFHeader))
	_, err = creator.ewfWriter.dest.Write(headerPad)
	if err != nil {
		return nil, err
	}

	numChunks := totalSize / DefaultChunkSize
	if totalSize%DefaultChunkSize > 0 {
		numChunks++
	}

	creator.AddDeviceInformation(EWF_DEVICE_INFO_BYTES_PER_SEC, "512")
	creator.AddDeviceInformation(EWF_DEVICE_INFO_NUMBER_OF_SECTORS, strconv.FormatInt(numChunks*64, 10))
	_, descN, err := creator.ewfWriter.Segment.DeviceInformation.Encode(creator.ewfWriter.dest, 0)
	if err != nil {
		return nil, err
	}

	creator.ewfWriter.previousDescriptorPosition = creator.ewfWriter.dest.position - int64(descN)

	creator.AddCaseData(EWF_CASE_DATA_NUMBER_OF_CHUNKS, strconv.FormatInt(numChunks, 10))
	creator.AddCaseData(EWF_CASE_DATA_NUMBER_OF_SECTORS_PC, "64")
	creator.AddCaseData(EWF_CASE_DATA_ERROR_GRANULARITY, "64")
	_, descN, err = creator.ewfWriter.Segment.CaseData.Encode(creator.ewfWriter.dest, creator.ewfWriter.previousDescriptorPosition)
	if err != nil {
		return nil, err
	}
	creator.ewfWriter.previousDescriptorPosition = creator.ewfWriter.dest.position - int64(descN)

	return creator.ewfWriter, nil
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
	bufc, err = ewf.compressor.Compress(p)
	if err != nil {
		return
	}

	// compression has bigger output
	flag := EWF_CHUNK_DATA_FLAG_IS_COMPRESSED
	if len(bufc) > len(p) {
		bufc = p
		flag = 0
	}

	cpos := ewf.dest.position
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

	ewf.Segment.addTableEntry(cpos, uint32(len(bufc)), uint32(flag))

	_, err = ewf.md5Hasher.Write(p)
	if err != nil {
		return
	}

	_, err = ewf.sha1Hasher.Write(p)
	return
}
