package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/asalih/go-ewf"
	"www.velocidex.com/golang/go-ntfs/parser"
)

func main() {

	e01Source, err := os.Open("./testdata/ezero1.vhd")
	// e01Source, err := os.Open("./testdata/ahmetsalih_vhd11.001")
	if err != nil {
		log.Fatalf("%v", err)
	}

	st, err := e01Source.Stat()
	if err != nil {
		log.Fatalf("%v", err)
	}

	// this source has boot part, i need to skip where ntfs starts
	offset := 65536
	// fmt.Println("Size: ", st.Size(), st.Size()-int64(offset))
	vhdNtfsRdr := io.NewSectionReader(e01Source, int64(offset), st.Size()-int64(offset))
	_ = vhdNtfsRdr

	e01ImageFile, err := os.Create("./testdata/testimage.E01")
	if err != nil {
		log.Fatalf("%v", err)
	}

	ewfc, err := ewf.CreateEWF(e01ImageFile)
	if err != nil {
		fmt.Println(err)
	}

	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_CASE_NUMBER, " ")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER, " ")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_ACQUIRY_SOFTWARE_VERSION, "ADI4.7.1.2")
	// ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_ACQUIRY_DATE, "2024 3 23 22 16 00")
	// ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_SYSTEM_DATE, "2024 3 23 22 16 00")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_ACQUIRY_DATE, "2024 3 12 14 27 31")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_SYSTEM_DATE, "2024 3 12 14 27 31")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_PASSWORD, "0")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_DESCRIPTION, "untitled")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_EXAMINER_NAME, " ")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_NOTES, " ")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_ACQUIRY_OPERATING_SYSTEM, "Win 201x")
	ewfc.AddMediaInfo(ewf.EWF_HEADER_VALUES_INDEX_COMPRESSION_TYPE, "f")

	//TODO(ahmet): start should return writer?
	err = ewfc.Start()
	if err != nil {
		fmt.Println(err)
	}

	_, err = copyBuffer(ewfc, vhdNtfsRdr, make([]byte, 1024*1024))
	if err != nil {
		log.Fatal(err)
	}

	err = ewfc.Close()
	if err != nil {
		fmt.Println(err)
	}

	/* ------- READERS --------*/

	// e01Files, err := filepath.Glob("./testdata/esifirbir.E01")
	// e01Files, err := filepath.Glob("./testdata/The Janitor.E011")
	// e01Files, err := filepath.Glob("./testdata/The Janitor Copy.E01")
	e01Files, err := filepath.Glob("./testdata/testimage.E01")
	// e01Files, err := filepath.Glob("./testdata/multiseg/rand.dd.*")
	// e01Files, err := filepath.Glob("./testdata/test.ntfs.dd.E01")
	if err != nil {
		log.Fatalf("%v", err)
	}
	if len(e01Files) == 0 {
		log.Fatal("e01 file not found")
	}

	fhs := make([]io.ReadSeeker, 0)
	for _, f := range e01Files {
		file, err := os.Open(f)
		if err != nil {
			log.Fatalf("%v", err)
		}
		fhs = append(fhs, file)
	}

	ewfImg, err := ewf.OpenEWF(fhs...)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Println("Size: ", ewfImg.Size)

	ntfs_ctx, err := parser.GetNTFSContext(ewfImg, 0)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	ls(ntfs_ctx, "/")

}

func GetMFTEntry(ntfs_ctx *parser.NTFSContext, filename string) (*parser.MFT_ENTRY, error) {
	mft_idx, _, _, err := parser.ParseMFTId(filename)
	if err == nil {
		// Access by mft id (e.g. 1234-128-6)
		return ntfs_ctx.GetMFT(mft_idx)
	} else {
		// Access by filename.
		dir, err := ntfs_ctx.GetMFT(5)
		if err != nil {
			return nil, err
		}

		return dir.Open(ntfs_ctx, filename)
	}
}

func ls(ntfs_ctx *parser.NTFSContext, path string) {
	fmt.Println("working ls: ", path)

	dir, err := GetMFTEntry(ntfs_ctx, path)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	for _, info := range parser.ListDir(ntfs_ctx, dir) {
		child_entry, err := GetMFTEntry(ntfs_ctx, info.MFTId)
		if err != nil {
			log.Fatalf("%+v", err)
		}

		full_path, err := parser.GetFullPath(ntfs_ctx, child_entry)
		if err != nil {
			log.Fatalf("%+v", err)
		}

		fmt.Println([]string{
			info.MFTId,
			full_path,
			fmt.Sprintf("%v", info.Size),
			fmt.Sprintf("%v", info.Mtime.In(time.UTC)),
			fmt.Sprintf("%v", info.IsDir),
			info.Name,
		})
	}
}

func copyBuffer(dst *ewf.EWFWriter, src io.Reader, buf []byte) (written int64, err error) {
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.WriteData(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
