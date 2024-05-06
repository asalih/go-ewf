package ewf

import (
	"bytes"
	"compress/zlib"
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
	binary.Read(buff, binary.LittleEndian, &u16)
	return string(utf16.Decode(u16))
}

func ToMap[K comparable, T any](keys []K, vals []T) map[K]T {
	m := map[K]T{}

	for idx, key := range keys {
		m[key] = vals[idx]
	}
	return m
}

func decompress(val []byte) ([]byte, error) {
	b := bytes.NewReader(val)

	zr, err := zlib.NewReader(b)
	if err != nil {
		if err != io.EOF {
			return nil, err
		}
		err = nil
	}
	defer zr.Close()

	return io.ReadAll(zr)
}

func compress(val []byte) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	wr, err := zlib.NewWriterLevel(buf, zlib.BestSpeed)
	if err != nil {
		return nil, err
	}
	_, err = wr.Write(val)
	if err != nil {
		_ = wr.Close()
		return nil, err
	}
	err = wr.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func getElementAtIndex(l *list.List, index int) (*list.Element, bool) {
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
