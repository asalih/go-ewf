package shared

import (
	"bytes"
	"compress/bzip2"
	"compress/zlib"
	"io"
)

type Decompressor func(val []byte) ([]byte, error)

func SkipDecompress(val []byte) ([]byte, error) {
	return val, nil
}

func DecompressBZip2(val []byte) ([]byte, error) {
	zr := bzip2.NewReader(bytes.NewReader(val))

	return io.ReadAll(zr)
}

func DecompressZlib(val []byte) ([]byte, error) {
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

func CompressZlib(val []byte) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	wr, err := zlib.NewWriterLevel(buf, zlib.BestCompression)
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
