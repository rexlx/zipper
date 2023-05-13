archive a directory to your minio object storage
```
go build -o zipper ./main.go
# ./zipper -src SOURCE -dst DEST.zip (all trailing args are directories to exclude)
./zipper -src /Users/rxlx -dst /Volumes/s/weekly.zip Library .docker Movies
```