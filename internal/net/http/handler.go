package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultReadTimeout    = 100 * time.Second // Cloudflare's default read request timeout of 100s
	DefaultWriteTimeout   = 30 * time.Second  // Cloudflare's default write request timeout of 30s
	DefaultIdleTimeout    = 900 * time.Second // Cloudflare's default write request timeout of 900s
	DefaultMaxHeaderBytes = 32 * (1 << 10)
	DefaultMaxBytes       = 1 << 20 // Cloudflare's free tier limits of 100mb
)

type UpDownloader[K fmt.Stringer] interface {
	Uploader[K]
	Downloader
}

func Handler(up UpDownloader[uuid.UUID]) http.Handler {
	mux := http.NewServeMux()
	handleFunc := func(pattern string, h http.Handler) {
		mux.Handle(pattern, h)
	}
	handleFunc("GET /ready", statusHandler{code: 200})

	// use versioning in headers rather than paths?
	handleFunc("POST /cloud-storage/files", handleUploadCloudStorage(up))
	handleFunc("GET /cloud-storage/files/{file}", handleDownloadCloudStorage(up))

	h := AcceptHandler(mux)
	return h
}
