package main

import (
	"log"
	"os"
	"path/filepath"
)

// file obj stored in the torrent file
type TorrentFileObj struct {
	Length     int64
	Path       []string
	JoinedPath string
}

type TorrentDir struct {
	Files   []TorrentFileObj
	Dirname string
}

func IntToBits(n int, width int) []int {
	bits := make([]int, width)

	for i := 0; i < width; i++ {
		if (n & (1 << (width - 1 - i))) != 0 {
			bits[i] = 1
		}
	}

	return bits
}

func util_max(a int64, b int64) int64 {
	if a >= b {
		return a

	} else {
		return b
	}
}

func util_min(a int64, b int64) int64 {
	if a <= b {
		return a

	} else {
		return b
	}
}

func CreateFileWithSize(filename string, size int64) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
	if err != nil {
		// if file exists, just return
		if os.IsExist(err) {
			return nil
		}
		log.Println("E: Creating file.", err.Error())
		return err
	}
	defer f.Close()

	err = f.Truncate(size)
	if err != nil {
		log.Println("E: Truncating file to size.", err.Error())
		return err
	}

	_, err = f.Seek(size-1, 0)
	if err != nil {
		log.Println("E: Seeking file.", err.Error())
		return err
	}

	_, err = f.Write([]byte{0})
	if err != nil {
		log.Println("E: Writing to file.", err.Error())
		return err
	}

	return nil
}

func CreateAllFiles(td TorrentDir) error {
	for i := range len(td.Files) {
		// rootdir + rest of the pathparts
		joined_path := filepath.Join(append([]string{td.Dirname}, td.Files[i].Path...)...)

		td.Files[i].JoinedPath = joined_path

		if err := os.MkdirAll(filepath.Dir(joined_path), 0755); err != nil {
			log.Println("E: Making file dir", err.Error())
			return err
		}

		if err := CreateFileWithSize(joined_path, td.Files[i].Length); err != nil {
			return err
		}
	}

	return nil
}
