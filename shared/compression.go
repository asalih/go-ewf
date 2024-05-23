package shared

import (
	"bytes"
	"compress/bzip2"
	"compress/zlib"
	"io"
	"sync"
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
	if err != nil && err != io.EOF {
		return nil, err
	}
	defer zr.Close()

	return io.ReadAll(zr)
}

type ZlibCompressor struct {
	mu sync.Mutex

	buf *bytes.Buffer
	wr  *zlib.Writer
}

func NewZlibCompressor() (*ZlibCompressor, error) {
	buf := bytes.NewBuffer(nil)
	wr, err := zlib.NewWriterLevel(buf, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	return &ZlibCompressor{
		buf: buf,
		wr:  wr,
	}, nil
}

func (c *ZlibCompressor) Reset() {
	c.buf.Reset()
	c.wr.Reset(c.buf)
}

func (c *ZlibCompressor) Compress(val []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.Reset()

	// Write the input data to the zlib writer
	_, err := c.wr.Write(val)
	if err != nil {
		_ = c.wr.Close()
		return nil, err
	}

	// Close the writer to flush the compressed data
	err = c.wr.Close()
	if err != nil {
		return nil, err
	}

	// Get the compressed data
	return c.buf.Bytes(), nil

}
