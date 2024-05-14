package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/asalih/go-ewf/evf1"
	"github.com/asalih/go-ewf/evf2"
	"www.velocidex.com/golang/go-ntfs/parser"
)

func main() {

	e01Source, err := os.Open("./testdata/ezero1.vhd")
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

	e01ImageFile, err := os.Create("./testdata/testimage.Ex01")
	if err != nil {
		log.Fatalf("%v", err)
	}

	ewfc, err := evf2.CreateEWF(e01ImageFile)
	if err != nil {
		fmt.Println(err)
	}

	ewfc.AddCaseData("cn", "")
	ewfc.AddCaseData("ex", "")
	ewfc.AddCaseData("nt", "")
	ewfc.AddCaseData("av", "20240506")
	ewfc.AddCaseData("at", "1715269156")
	ewfc.AddCaseData("sb", "64")
	ewfc.AddCaseData("en", "")
	ewfc.AddCaseData("os", "Windows")
	ewfc.AddCaseData("nm", "")
	ewfc.AddCaseData("tb", "2047")
	ewfc.AddCaseData("wb", "")
	ewfc.AddCaseData("tt", "")
	ewfc.AddCaseData("cp", "")
	ewfc.AddCaseData("gr", "")

	ewfc.AddDeviceInformation("md", "")
	ewfc.AddDeviceInformation("dc", "")
	ewfc.AddDeviceInformation("pid", "")
	ewfc.AddDeviceInformation("ls", "")
	ewfc.AddDeviceInformation("sn", "")
	ewfc.AddDeviceInformation("lb", "")
	ewfc.AddDeviceInformation("hs", "")
	ewfc.AddDeviceInformation("dt", "f")
	ewfc.AddDeviceInformation("rs", "")
	ewfc.AddDeviceInformation("bp", "512")
	ewfc.AddDeviceInformation("ph", "1")

	err = ewfc.Start(uint64(st.Size() - int64(offset)))
	if err != nil {
		fmt.Println(err)
	}

	_, err = io.CopyBuffer(ewfc, vhdNtfsRdr, make([]byte, 1024*1024))
	if err != nil {
		log.Fatal(err)
	}

	err = ewfc.Close()
	if err != nil {
		fmt.Println(err)
	}

	// e01Files, err := filepath.Glob("./testdata/libewf-ec7v2-ext4.Ex01")
	// e01Files, err := filepath.Glob("./testdata/imagex.Ex01")
	e01Files, err := filepath.Glob("./testdata/testimage.Ex01")
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

	ewfImg, err := evf2.OpenEWF(fhs...)
	if err != nil {
		log.Fatalf("%v", err)
	}

	ntfs_ctx, err := parser.GetNTFSContext(ewfImg, 0)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	ls(ntfs_ctx, "/")

}

func GetMFTEntry(ntfs_ctx *parser.NTFSContext, filename string) (*parser.MFT_ENTRY, error) {
	mft_idx, _, _, _, err := parser.ParseMFTId(filename)
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

		full_path := parser.GetFullPath(ntfs_ctx, child_entry)

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

func mainv1() {

	e01Source, err := os.Open("./testdata/ezero1.vhd")
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

	ewfc, err := evf1.CreateEWF(e01ImageFile)
	if err != nil {
		fmt.Println(err)
	}

	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_CASE_NUMBER, " ")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EVIDENCE_NUMBER, " ")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_ACQUIRY_SOFTWARE_VERSION, "ADI4.7.1.2")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_ACQUIRY_DATE, "2024 3 12 14 27 31")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_SYSTEM_DATE, "2024 3 12 14 27 31")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_PASSWORD, "0")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_DESCRIPTION, "untitled")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_EXAMINER_NAME, " ")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_NOTES, " ")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_ACQUIRY_OPERATING_SYSTEM, "Win 201x")
	ewfc.AddMediaInfo(evf1.EWF_HEADER_VALUES_INDEX_COMPRESSION_TYPE, "f")

	//TODO(ahmet): start should return writer?
	err = ewfc.Start()
	if err != nil {
		fmt.Println(err)
	}

	_, err = io.CopyBuffer(ewfc, vhdNtfsRdr, make([]byte, 1024*1024))
	if err != nil {
		log.Fatal(err)
	}

	err = ewfc.Close()
	if err != nil {
		fmt.Println(err)
	}

	/* ------- READERS --------*/

	e01Files, err := filepath.Glob("./testdata/testimage.E01")
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

	ewfImg, err := evf1.OpenEWF(fhs...)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Println("Size: ", ewfImg.Size)

}
