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
	"runtime"
	"strings"
	"time"

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
	Slash       string
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

	zpr := Zipper{
		URL:    "storage.nullferatu.com:9000",
		ID:     "RMCMjDSBEUnHT0vO",
		Key:    "OjtUEjI0KEWLCtvDIYy6DwFQ8Pgo6D3g",
		Bucket: "backup",
		Source: *src,
		SSL:    false,
	}

	archive := zpr.init()

	mc, err := minio.New(zpr.URL, &minio.Options{
		Creds:  credentials.NewStaticV4(zpr.ID, zpr.Key, ""),
		Secure: zpr.SSL,
	})
	if err != nil {
		log.Fatalln("could not create minio client, exiting")
	}

	err = zpr.zip(archive, zpr.Source, ignore)
	if err != nil {
		log.Fatalln("failed when zipping, exiting", err)
	}

	err = zpr.save(mc)
	if err != nil {
		log.Fatalln("failed when zipping, exiting", err)
	}

}

func (z *Zipper) init() *os.File {
	switch dist := runtime.GOOS; dist {
	case "windows":
		z.Slash = "\\"
	default:
		z.Slash = "/"
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln("couldnt get hostname, exiting")
	}

	if *dst == "" {
		z.ObjName = fmt.Sprintf("%v_bak.zip", hostname)
		z.Destination = fmt.Sprintf("%v_bak.zip", hostname)
	} else {
		z.ObjName = fmt.Sprintf("%v_%v", hostname, *dst)
		z.Destination = *dst
	}
	arc, err := os.Create(z.Destination)
	if err != nil {
		log.Fatalln("could not create archive, exiting")
	}
	return arc
}

func (z *Zipper) save(mc *minio.Client) error {
	start := time.Now()
	res, err := mc.FPutObject(
		context.Background(),
		z.Bucket,
		z.ObjName,
		z.Destination,
		minio.PutObjectOptions{ContentType: "application/zip"})
	if err != nil {
		return err
	}
	end := time.Now()
	log.Printf("moved: %v MiB in %v", float32(res.Size)/float32(MiB), end.Sub(start))
	err = os.Remove(z.Destination)
	if err != nil {
		return err
	}
	return nil
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
		if z.fileExists(path, ignore) {
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
	log.Println("Compressed folder successfully.")
	return nil
}

func (z *Zipper) fileExists(filename string, filesToIgnore []string) bool {
	for _, i := range filesToIgnore {
		if strings.Contains(filename, fmt.Sprintf("%v%v", z.Slash, i)) {
			return false
		}
	}
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
