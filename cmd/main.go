package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/asalih/ewf"
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

	ewfImg, err := ewf.NewEWF(f)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Println("Size: ", ewfImg.Size)
}
