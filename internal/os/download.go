package os

import (
	"context"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type Downloader struct {
	bucket string
	m      *manager.Downloader // just have this as a global atomic?
}

func (d *Downloader) Download(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	s := strings.Replace(id.String(), "-", "", 4)
	uri := path.Join("_blob", s[:2], s[2:4], s[4:])

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		o := &s3.GetObjectInput{
			Key:    &uri,
			Bucket: &d.bucket,
			// PartNumber *string
			// Range *string
		}
		_, err := d.m.Download(ctx, &writerAt{pw}, o)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
	}()
	return pr, nil
}

func NewDownloader(bucket string, c manager.DownloadAPIClient) *Downloader {
	d := manager.NewDownloader(c, func(u *manager.Downloader) {
		u.PartSize = 1 << 24 // 16MB
		// https://dev.to/flowup/using-io-reader-io-writer-in-go-to-stream-data-3i7b
		u.Concurrency = 1 // force sequential writes
	})
	return &Downloader{bucket, d}
}
