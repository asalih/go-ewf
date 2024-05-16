package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/asalih/go-ewf/evf1"
	"github.com/asalih/go-ewf/evf2"
)

func main() {

	sourcePath := flag.String("source", "", "Source path")
	sourceOffset := flag.Int("offset", 0, "Source offset")
	targetPath := flag.String("target", "", "Target path")
	format := flag.String("format", "v2", "Format; v1/v2")

	flag.Parse()

	fmt.Println("EWF started: ", os.Args)
	switch *format {
	case "v1":
		err := handleV1(*sourcePath, *sourceOffset, *targetPath)
		if err != nil {
			log.Fatalf("%v", err)
		}
	case "v2":
		err := handleV2(*sourcePath, *sourceOffset, *targetPath)
		if err != nil {
			log.Fatalf("%v", err)
		}
	default:
		log.Fatalf("invalid format")
	}
	fmt.Println("EWF completed")
}

func open(path string, offset int) (io.ReadSeeker, error) {
	e01Source, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	st, err := e01Source.Stat()
	if err != nil {
		return nil, err
	}

	rdr := io.NewSectionReader(e01Source, int64(offset), st.Size()-int64(offset))
	return rdr, nil
}

func handleV2(sourcePath string, sourceOffset int, targetPath string) error {
	sourceReader, err := open(sourcePath, sourceOffset)
	if err != nil {
		return err
	}

	e01ImageFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	ewfc, err := evf2.CreateEWF(e01ImageFile)
	if err != nil {
		return err
	}

	sz, err := sourceReader.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	_, err = sourceReader.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = ewfc.Start(sz)
	if err != nil {
		return err
	}

	_, err = io.CopyBuffer(ewfc, sourceReader, make([]byte, 1024*1024))
	if err != nil {
		return err
	}

	err = ewfc.Close()
	if err != nil {
		return err
	}

	return nil
}

func handleV1(sourcePath string, sourceOffset int, targetPath string) error {
	sourceReader, err := open(sourcePath, sourceOffset)
	if err != nil {
		return err
	}

	e01ImageFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	ewfc, err := evf1.CreateEWF(e01ImageFile)
	if err != nil {
		return err
	}

	err = ewfc.Start()
	if err != nil {
		return err
	}

	_, err = io.CopyBuffer(ewfc, sourceReader, make([]byte, 1024*1024))
	if err != nil {
		return err
	}

	err = ewfc.Close()
	if err != nil {
		return err
	}

	return nil
}
