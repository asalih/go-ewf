package evf2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/asalih/go-ewf/shared"
)

type EWFDeviceInformationKey string

const (
	EWF_DEVICE_INFO_SERIAL_NUMBER        EWFDeviceInformationKey = "sn"
	EWF_DEVICE_INFO_DRIVE_MODEL          EWFDeviceInformationKey = "md"
	EWF_DEVICE_INFO_DRIVE_LABEL          EWFDeviceInformationKey = "lb"
	EWF_DEVICE_INFO_NUMBER_OF_SECTORS    EWFDeviceInformationKey = "ts"
	EWF_DEVICE_INFO_NUMBER_OF_HPA        EWFDeviceInformationKey = "hs"
	EWF_DEVICE_INFO_NUMBER_OF_DCO        EWFDeviceInformationKey = "dc"
	EWF_DEVICE_INFO_DRIVE_TYPE           EWFDeviceInformationKey = "dt"
	EWF_DEVICE_INFO_PROCESS_ID           EWFDeviceInformationKey = "pid"
	EWF_DEVICE_INFO_MUMBER_OF_PALM       EWFDeviceInformationKey = "rs"
	EWF_DEVICE_INFO_NUMBER_OF_SMART_LOGS EWFDeviceInformationKey = "ls"
	EWF_DEVICE_INFO_BYTES_PER_SEC        EWFDeviceInformationKey = "bp"
	EWF_DEVICE_INFO_IS_PHYSICAL          EWFDeviceInformationKey = "ph"
)

var DeviceInformationIdentifiers = map[EWFDeviceInformationKey]string{
	EWF_DEVICE_INFO_SERIAL_NUMBER:        "Serial Number",
	EWF_DEVICE_INFO_DRIVE_MODEL:          "Drive Model",
	EWF_DEVICE_INFO_DRIVE_LABEL:          "Drive Label",
	EWF_DEVICE_INFO_NUMBER_OF_SECTORS:    "Number of Sectors",
	EWF_DEVICE_INFO_NUMBER_OF_HPA:        "Number of HPA protected sectors",
	EWF_DEVICE_INFO_NUMBER_OF_DCO:        "Number of DCO protected sectors",
	EWF_DEVICE_INFO_DRIVE_TYPE:           "Drive Type",
	EWF_DEVICE_INFO_PROCESS_ID:           "Process Identifier",
	EWF_DEVICE_INFO_MUMBER_OF_PALM:       "Number of sectors PALM Ram device",
	EWF_DEVICE_INFO_NUMBER_OF_SMART_LOGS: "SMART or ATA logs",
	EWF_DEVICE_INFO_BYTES_PER_SEC:        "Bytes per Sector",
	EWF_DEVICE_INFO_IS_PHYSICAL:          "Is physical",
}

type EWFDeviceInformationSection struct {
	NumberOfObjects string
	ObjectName      string
	KeyValue        map[string]string
}

func (ewfHeader *EWFDeviceInformationSection) Decode(fh io.ReadSeeker, section *EWFSectionDescriptor, decompressorFunc shared.Decompressor) error {
	fh.Seek(section.DataOffset, io.SeekStart)
	rd := make([]byte, section.Size)
	if _, err := fh.Read(rd); err != nil {
		return err
	}

	data, err := decompressorFunc(rd)
	if err != nil {
		return err
	}

	// Starts with a BOM
	if data[0] == 255 || data[1] == 254 {
		data = []byte(shared.UTF16ToUTF8(data))
	}

	var identifiers []string
	var values []string

	for lineNum, line := range bytes.Split(data, newLineDelim) {
		for _, attr := range bytes.Split(line, fieldDelim) {
			strAttr := string(bytes.TrimSuffix(attr, []byte{'\r'}))
			if lineNum == 0 {
				ewfHeader.NumberOfObjects = string(strAttr[0])
			} else if lineNum == 1 {
				ewfHeader.ObjectName = strAttr
			} else if lineNum == 2 {
				_, ok := DeviceInformationIdentifiers[EWFDeviceInformationKey(strAttr)]
				if !ok {
					return fmt.Errorf("media identifier is unknown: %s", strAttr)
				}
				identifiers = append(identifiers, strAttr)
			} else if lineNum == 3 {
				values = append(values, strAttr)
			}
		}
	}
	ewfHeader.KeyValue = shared.ToMap(identifiers, values)

	return nil
}

// Encode writes data and its description to the target writer. Returns  data write count, descriptor write count and err
func (ewfHeader *EWFDeviceInformationSection) Encode(ewf io.Writer, previousDescriptorPosition int64) (dataN int, descN int, err error) {
	buf := bytes.NewBuffer(nil)

	buf.WriteString(ewfHeader.NumberOfObjects)
	buf.Write(newLineDelim)
	buf.WriteString(ewfHeader.ObjectName)
	buf.Write(newLineDelim)

	mk := make([]string, 0, len(ewfHeader.KeyValue))
	mv := make([]string, 0, len(ewfHeader.KeyValue))
	for k, v := range ewfHeader.KeyValue {
		mk = append(mk, k)
		mv = append(mv, v)
	}
	buf.WriteString(strings.Join(mk, string(fieldDelim)))
	buf.Write(newLineDelim)
	buf.WriteString(strings.Join(mv, string(fieldDelim)))
	buf.Write(newLineDelim)
	buf.Write(newLineDelim)

	utf16Data := shared.UTF8ToUTF16(buf.Bytes())
	zlHeader, err := shared.CompressZlib(utf16Data)
	if err != nil {
		return 0, 0, err
	}

	zlHeader, paddingSize := alignTo16Bytes(zlHeader)

	dataN, err = ewf.Write(zlHeader)
	if err != nil {
		return 0, 0, err
	}

	desc := NewEWFSectionDescriptorData(EWF_SECTION_TYPE_DEVICE_INFORMATION)
	desc.DataSize = uint64(len(zlHeader))
	desc.PreviousOffset = uint64(previousDescriptorPosition)
	desc.PaddingSize = uint32(paddingSize)

	// header section and its data appears twice subsequently
	// after first write, we arrange the "Next" field then write
	descN, chc, err := shared.WriteWithSum(ewf, desc)
	if err != nil {
		return 0, 0, err
	}
	desc.Checksum = chc
	return dataN, descN, err
}

func (c *EWFDeviceInformationSection) GetSectorSize() (int, error) {
	bp, ok := c.KeyValue[string(EWF_DEVICE_INFO_BYTES_PER_SEC)]
	if !ok {
		return 0, errors.New("device info has no sector size")
	}
	return strconv.Atoi(strings.TrimSpace(bp))
}
