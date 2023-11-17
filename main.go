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
	Start        time.Time `json:"start"`
	FilesCopied  int64     `json:"files_copied"`
	TotalWritten int64     `json:"total_written"`
	Source       string    `json:"source"`
	Destination  string    `json:"destination"`
	URL          string    `json:"url"`
	ID           string    `json:"id"`
	Key          string    `json:"key"`
	Bucket       string    `json:"bucket"`
	ObjName      string    `json:"obj_name"`
	Slash        string    `json:"slash"`
	SSL          bool      `json:"ssl"`
}

// you can replace the url/id/key with your own s3 compatible server info as this is just an example
// of how to hard code the values into flags. Or keep as is and pass the values you'd like as args...
// the defualts do not work.
var (
	src    = flag.String("src", "", "source dir")
	dst    = flag.String("dst", "", "zip location")
	bucket = flag.String("bucket", "backup", "s3 bucket")
	url    = flag.String("url", "cobra.nullferatu.com:9000", "s3 url")
	s3Id   = flag.String("id", "fgw4Mwn24aj3XIdrIHb6", "s3 id")
	s3Key  = flag.String("key", "4MGexyqyXoduYkCGL6WJaOr2FMowmZ2ObQyCnGOf", "s3 key")
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
		Start:  time.Now(),
		URL:    *url,
		ID:     *s3Id,
		Key:    *s3Key,
		Bucket: *bucket,
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

	log.Println("Finished in", time.Since(zpr.Start))

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
	log.Printf("moved: %v MiB in %v", float32(res.Size)/float32(MiB), time.Since(start))
	err = os.Remove(z.Destination)
	if err != nil {
		return err
	}
	return nil
}

func (z *Zipper) zip(archive *os.File, targetPath string, ignore []string) error {
	go func() {
		for {
			time.Sleep(3 * time.Second)
			fmt.Printf("%v Copied %v files (%v)...\r", time.Now(), z.FilesCopied, TotalWrittenToHumanReadable(z.TotalWritten))
		}
	}()
	zw := zip.NewWriter(archive)
	// defer zw.Close()
	if err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil && !os.IsPermission(err) {
			return err
		}

		if err != nil && os.IsPermission(err) {
			// log.Println("Skipping file due to permission error:", path)
			return nil
		}

		// Add the file to the zip archive.
		relPath := strings.TrimPrefix(path, filepath.Dir(targetPath))
		zfh, err := zw.Create(relPath)
		if err != nil {
			return err
		}
		// fmt.Println(path)
		if z.fileExists(path, ignore) {
			fsFile, err := os.Open(path)
			if err != nil && !os.IsPermission(err) {
				return err
			}
			written, err := io.Copy(zfh, fsFile)
			if err != nil {
				log.Println("hmm..", err, zfh, fsFile)
			}
			z.TotalWritten += written
			z.FilesCopied++
		}
		return nil
	}); err != nil && !os.IsPermission(err) {
		return err
	}

	// Close the zip writer.
	zw.Close()

	// Success!
	log.Println("Compressed folder successfully, uploading...")
	return nil
}

func (z *Zipper) fileExists(filename string, filesToIgnore []string) bool {
	for _, i := range filesToIgnore {
		// fmt.Println(filename, i)
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

func TotalWrittenToHumanReadable(totalWritten int64) string {
	if totalWritten < KiB {
		return fmt.Sprintf("%dB", totalWritten)
	}
	if totalWritten < MiB {
		return fmt.Sprintf("%.2fKiB", float32(totalWritten)/float32(KiB))
	}
	if totalWritten < GiB {
		return fmt.Sprintf("%.2fMiB", float32(totalWritten)/float32(MiB))
	}
	if totalWritten < TiB {
		return fmt.Sprintf("%.2fGiB", float32(totalWritten)/float32(GiB))
	}
	return fmt.Sprintf("%.2fTiB", float32(totalWritten)/float32(TiB))
}
