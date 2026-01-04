package shared

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"hash/adler32"
	"io"
	"unicode/utf16"
)

const adler32SumSize = 4

// WriteWithSum specifically objects which ends with Checksum field and writes to target by calculating adler32 sum
func WriteWithSum(dest io.Writer, obj interface{}) (n int, sum uint32, err error) {
	buf := bytes.NewBuffer(nil)
	err = binary.Write(buf, binary.LittleEndian, obj)
	if err != nil {
		return
	}

	data := buf.Bytes()
	data = data[:len(data)-adler32SumSize]
	sum = adler32.Checksum(data)

	n, err = dest.Write(data)
	if err != nil {
		return
	}
	err = binary.Write(dest, binary.LittleEndian, sum)
	if err != nil {
		return
	}
	n += adler32SumSize

	return
}

func UTF16ToUTF8(in []byte) string {
	buff := bytes.NewReader(in)
	u16 := make([]uint16, len(in)/2)
	_ = binary.Read(buff, binary.LittleEndian, &u16)
	return string(utf16.Decode(u16))
}
func UTF8ToUTF16(utf8Bytes []byte) []byte {
	str := string(utf8Bytes)
	utf16Runes := utf16.Encode([]rune(str))
	// Convert utf16Runes to []byte
	byteBuffer := []byte{0xFF, 0xFE}
	for _, r := range utf16Runes {
		byteBuffer = append(byteBuffer, byte(r&0xFF), byte(r>>8))
	}

	return byteBuffer
}

func ToMap[K comparable, T any](keys []K, vals []T) map[K]T {
	m := map[K]T{}

	for idx, key := range keys {
		m[key] = vals[idx]
	}
	return m
}

func GetListElement(l *list.List, index int) (*list.Element, bool) {
	if index < 0 || index >= l.Len() {
		return nil, false // Index out of bounds
	}
	var element *list.Element
	if index < l.Len()/2 { // Optimize traversal direction based on index
		element = l.Front()
		for i := 0; i < index; i++ {
			element = element.Next()
		}
	} else {
		element = l.Back()
		for i := l.Len() - 1; i > index; i-- {
			element = element.Prev()
		}
	}
	return element, true
}

func PadBytes(buf []byte, targetLen int) []byte {
	currentLength := len(buf)
	if currentLength >= targetLen {
		return buf
	}

	padding := make([]byte, targetLen-currentLength)
	return append(buf, padding...)
}

func MinUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
