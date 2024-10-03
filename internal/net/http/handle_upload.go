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
	Upload(ctx context.Context, r io.Reader) (id V, sz int64, err error)
}

func handleUploadBlob[V any](up Uploader[V]) http.HandlerFunc {
	var unsupportedMediaType = statusHandler{
		code: http.StatusUnsupportedMediaType,
		s:    `request is not a mulitpart/form`,
	}

	var unprocessableEntity = func(format string, v ...any) statusHandler {
		return statusHandler{
			code: http.StatusUnprocessableEntity,
			s:    fmt.Sprintf(format, v...),
		}
	}

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
			unsupportedMediaType.ServeHTTP(w, r)
			return
		}
		part, err := mr.NextPart()
		if err != nil {
			unprocessableEntity("failed to decode part: %v", err).ServeHTTP(w, r)
			return
		}
		defer part.Close()
		// validate filename/formname
		filename := part.FileName()
		debug.Printf("%q := part.FileName()", filename)
		id, sz, err := up.Upload(ctx, part)
		if err != nil {
			// "failed to upload file: %v", err
			Error(w, r, err)
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
