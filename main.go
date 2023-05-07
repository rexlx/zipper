package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Zipper struct {
	URL     string
	ID      string
	Key     string
	Bucket  string
	ObjName string
	SSL     bool
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln("couldnt get hostname, exiting")
	}
	x := fmt.Sprintf("/Volumes/s/%v_bak.zip", hostname)
	zpr := Zipper{
		URL:     "storage.nullferatu.com:9000",
		ID:      "RMCMjDSBEUnHT0vO",
		Key:     "OjtUEjI0KEWLCtvDIYy6DwFQ8Pgo6D3g",
		Bucket:  "backup",
		ObjName: fmt.Sprintf("%v_bak.zip", hostname),
		SSL:     false,
	}
	mc, err := minio.New(zpr.URL, &minio.Options{
		Creds:  credentials.NewStaticV4(zpr.ID, zpr.Key, ""),
		Secure: zpr.SSL,
	})
	if err != nil {
		log.Fatalln("could not create minio, exiting")
	}
	path := "/Users/rxlx/"
	arc, err := os.Create(x)
	if err != nil {
		log.Fatalln("could not create archive, exiting")
	}
	err = Zip(arc, path)
	if err != nil {
		log.Fatalln("could not create archive, exiting", err)
	}
	res, err := mc.FPutObject(
		context.Background(),
		zpr.Bucket,
		zpr.ObjName,
		x,
		minio.PutObjectOptions{ContentType: "application/zip"})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("moved:", res.Size)
	err = os.Remove(x)
	if err != nil {
		log.Fatal("couldnt remove archive...", err)
	}

}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func Zip(archive *os.File, targetPath string) error {
	zw := zip.NewWriter(archive)
	defer zw.Close()
	if err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories.
		if info.IsDir() {
			return nil
		}

		// Add the file to the zip archive.
		relPath := strings.TrimPrefix(path, filepath.Dir(targetPath))
		zfh, err := zw.Create(relPath)
		if err != nil {
			return err
		}
		if strings.Contains(path, ".local/share/containers") {
			return nil
		}
		fsFile, err := os.Open(path)
		if err != nil {
			log.Println("open+:", err)
		}
		if fileExists(path) {
			_, err = io.Copy(zfh, fsFile)
			if err != nil {
				log.Println("hmm..", err)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// Close the zip writer.
	zw.Close()

	// Success!
	fmt.Println("Compressed folder successfully.")
	return nil
}
