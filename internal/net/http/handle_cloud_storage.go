package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.adoublef/blob/internal/runtime/debug"
)

type Uploader[K fmt.Stringer] interface {
	Upload(ctx context.Context, r io.Reader) (id K, sz int64, err error)
}

func handleUploadCloudStorage[V fmt.Stringer](up Uploader[V]) http.HandlerFunc {
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
			ID:      id.String(),
			Size:    sz,
			Elapsed: time.Since(start).String(),
		}
		err = json.NewEncoder(w).Encode(c)
		debug.Printf(`%v = json.NewEncoder(w).Encode(%#v)`, err, c)
	}
}

type Downloader interface {
	Download(ctx context.Context, id uuid.UUID) (rc io.ReadCloser, err error)
}

func handleDownloadCloudStorage(d Downloader) http.HandlerFunc {
	var badPathValue = statusHandler{
		code: http.StatusBadRequest,
		s:    `path parameter has invalid format`,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// query for attachment (default) or inline
		// "type;filename=somefile.ext"

		id, err := uuid.Parse(r.PathValue("file"))
		if err != nil {
			badPathValue.ServeHTTP(w, r)
			return
		}
		rc, err := d.Download(ctx, id)
		if err != nil {
			Error(w, r, err)
			return
		}
		defer rc.Close()

		// return this to the user as attatchment or inline?
		// serveContent Headers
		// 1. last-modified
		// 1. pre-conditions
		// 1. content-type
		// 1. content-range (ordered?)
		// 1. accept-ranges
		// 1. content-encoding
		// 1. content-length - w.Header().Set("Content-Length", strconv.FormatInt(sendSize, 10))
		// check "HEAD"
		// io.CopyN(w, sendContent, sendSize)
		// if I serve a range should omit 'disposition'
		// see: https://stackoverflow.com/a/1401619/4239443

		// norma encoding: Content-Disposition: attachment; filename="filename.jpg"
		// special encoding (RFC 5987): Content-Disposition: attachment; filename*="filename.jpg"

		if r.Method != http.MethodHead {
			io.Copy(w, rc)
		}
	}
}
