package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.adoublef/blob/internal/runtime/debug"
)

type Uploader[V any] interface {
	Upload(ctx context.Context, filename string, r io.Reader) (id V, sz int64, err error)
}

func handleUploadBlob[V any](up Uploader[V]) http.HandlerFunc {
	type completed struct {
		ID      string `json:"resourceId"`
		Size    int64  `json:"bytesWritten"`
		Elapsed string `json:"timeElapsed"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		start := time.Now()

		mr, err := r.MultipartReader()
		if err != nil {
			// unsupportedMediaType
			return
		}
		part, err := mr.NextPart()
		if err != nil {
			// unprocessableEntity
			return
		}
		defer part.Close()
		// validate filename/formname
		filename := part.FileName()
		id, sz, err := up.Upload(ctx, filename, part)
		if err != nil {
			// Error(w, r, error)
			return
		}
		// todo: render function
		c := completed{
			// use [fmt.Stringer] instead
			ID:      fmt.Sprintf("%v", id),
			Size:    sz,
			Elapsed: time.Since(start).String(),
		}
		err = json.NewEncoder(w).Encode(c)
		debug.Printf(`%v = json.NewEncoder(w).Encode(%#v)`, err, c)
	}
}

// discardUploader is a test helper that should techincally live only undet `_test.go` files.
type discardUploader struct{}

// discardUploader implements [Uploader]
func (*discardUploader) Upload(ctx context.Context, name string, r io.Reader) (bool, int64, error) {
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return false, n, err
	}
	return true, n, nil
}
