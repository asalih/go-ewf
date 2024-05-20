package shared

import "io"

type EWFReader interface {
	io.ReadSeeker
	io.ReaderAt
	Size() int64
	Metadata() map[string]interface{}
}

type EWFWriter interface {
	io.WriteCloser
}
