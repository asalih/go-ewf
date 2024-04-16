package ewf

import (
	"bytes"
	"compress/zlib"
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

/*CMF|FLG  0x78|  (FLG|CM)
CM 0-3 Compression method  8=deflate
CINFO 4-7 Compression info 7=32K window size only when CM=8
FLG 0-4 FCHECK  = CMF*256 + FLG multiple of 31 = 120*256==x mod 31 => x=156
5 FDICT 1=> DICT follows (DICT is the Adler-32 checksum  of this sequence of bytes )
6-7 FLEVEL compression level 0-3
9c = 1001 1100
FLEVEL 10
FDICT 0
FCHECK 12
ADLER32  algorithm is a 32-bit extension and improvement of the Fletcher algorithm,
A compliant decompressor must check CMF, FLG, and ADLER32,
*/

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

func ToMap[K comparable, T any](keys []K, vals []T) map[K]T {
	m := map[K]T{}

	for idx, key := range keys {
		m[key] = vals[idx]
	}
	return m
}
