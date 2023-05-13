package main

import (
	"archive/zip"
	"context"
	"flag"
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
	Source      string
	Destination string
	URL         string
	ID          string
	Key         string
	Bucket      string
	ObjName     string
	SSL         bool
}

var (
	src = flag.String("src", "", "source dir")
	dst = flag.String("dst", "", "zip location")
)

const (
	_   = 1 << (10 * iota)
	KiB //1024
	MiB
	GiB
	TiB
)

func main() {
	flag.Parse()
	ignore := flag.Args()

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln("couldnt get hostname, exiting")
	}

	zpr := Zipper{
		URL:    "storage.nullferatu.com:9000",
		ID:     "RMCMjDSBEUnHT0vO",
		Key:    "OjtUEjI0KEWLCtvDIYy6DwFQ8Pgo6D3g",
		Bucket: "backup",
		Source: *src,
		SSL:    false,
	}

	if *dst == "" {
		zpr.ObjName = fmt.Sprintf("%v_bak.zip", hostname)
		zpr.Destination = fmt.Sprintf("%v_bak.zip", hostname)
	} else {
		zpr.ObjName = fmt.Sprintf("%v_%v", hostname, *dst)
		zpr.Destination = *dst
	}

	mc, err := minio.New(zpr.URL, &minio.Options{
		Creds:  credentials.NewStaticV4(zpr.ID, zpr.Key, ""),
		Secure: zpr.SSL,
	})
	if err != nil {
		log.Fatalln("could not create minio, exiting")
	}

	arc, err := os.Create(zpr.Destination)
	if err != nil {
		log.Fatalln("could not create archive, exiting")
	}

	err = zpr.zip(arc, *src, ignore)
	if err != nil {
		log.Fatalln("failed when zipping, exiting", err)
	}

	err = zpr.save(mc)
	if err != nil {
		log.Fatalln("failed when zipping, exiting", err)
	}

}

func (z *Zipper) save(mc *minio.Client) error {
	res, err := mc.FPutObject(
		context.Background(),
		z.Bucket,
		z.ObjName,
		z.Destination,
		minio.PutObjectOptions{ContentType: "application/zip"})
	if err != nil {
		return err
	}
	log.Println("moved:", res.Size/MiB)
	err = os.Remove(z.Destination)
	if err != nil {
		return err
	}
	return nil
}

func fileExists(filename string, filesToIgnore []string) bool {
	for _, i := range filesToIgnore {
		if strings.Contains(filename, fmt.Sprintf("/%v", i)) {
			return false
		}
	}
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func (z *Zipper) zip(archive *os.File, targetPath string, ignore []string) error {
	zw := zip.NewWriter(archive)
	// defer zw.Close()
	if err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil && !os.IsPermission(err) {
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
		if fileExists(path, ignore) {
			fsFile, err := os.Open(path)
			if err != nil && !os.IsPermission(err) {
				return err
			}
			_, err = io.Copy(zfh, fsFile)
			if err != nil {
				log.Println("hmm..", err, zfh, fsFile)
			}
		}
		return nil
	}); err != nil && !os.IsPermission(err) {
		return err
	}

	// Close the zip writer.
	zw.Close()

	// Success!
	fmt.Println("Compressed folder successfully.")
	return nil
}
