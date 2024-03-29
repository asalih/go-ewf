package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/asalih/go-ewf"
)

func main() {
	e01Files, err := filepath.Glob("./testdata/*.E01")
	if err != nil {
		log.Fatalf("%v", err)
	}
	if len(e01Files) == 0 {
		log.Fatal("e01 file not found")
	}

	f, err := os.Open(e01Files[0])
	if err != nil {
		log.Fatalf("%v", err)
	}

	ewfImg, err := ewf.OpenEWF(f)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Println("Size: ", ewfImg.Size)

	buf := make([]byte, 1024)
	_, err = ewfImg.ReadAt(buf, 0)
	if err != nil {
		log.Fatalf("%v", err)
	}

}
