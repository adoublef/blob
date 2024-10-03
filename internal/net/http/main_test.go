package http_test

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	. "go.adoublef/blob/internal/net/http"
	"go.adoublef/blob/internal/net/http/httputil"
	"go.adoublef/blob/internal/net/nettest"
	ospkg "go.adoublef/blob/internal/os"
	"go.adoublef/blob/internal/os/ostest"
	"go.adoublef/blob/internal/testing/is"
)

//go:embed all:testdata/*
var embedFS embed.FS

type TestClient struct {
	*http.Client
	*nettest.Proxy
	testing.TB
}

func (tc *TestClient) PostFile(ctx context.Context, pattern string, filename string) (*http.Response, error) {
	f, err := embedFS.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to return file stat: %v", err)
	}

	pr, pw := io.Pipe()
	defer pr.Close()

	mw := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		defer mw.Close()

		part, err := mw.CreateFormFile("file", fi.Name())
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		// upload a single file
		// can this be extended to upload a directory?
		n, err := io.CopyN(part, f, fi.Size())
		tc.Logf(`%d, %v := io.CopyN(part, f, fi.Size())`, n, err)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	o := func(r *http.Request) {
		// set contentType
		r.Header.Set("Content-Type", mw.FormDataContentType())
		// set accept
		r.Header.Set("Accept", "*/*")
		// set encoding?
	}
	return tc.Do(ctx, pattern, pr, o)
}

func (tc *TestClient) Do(ctx context.Context, pattern string, body io.Reader, opts ...func(*http.Request)) (*http.Response, error) {
	method, _, path, err := httputil.ParsePattern(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pattern: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to return request: %v", err)
	}
	tc.Logf(`req, err := http.NewRequestWithContext(ctx, %q, %q, body)`, method, path)
	for _, o := range opts {
		o(req)
	}
	res, err := tc.Client.Do(req)
	tc.Logf(`res, %v := tc.Client.Do(req)`, err)
	return res, err
}

func newTestClient(tb testing.TB, h http.Handler) *TestClient {
	tb.Helper()

	ts := httptest.NewUnstartedServer(h)
	ts.Config.MaxHeaderBytes = DefaultMaxHeaderBytes
	// note: the client panics if readTimeout is less than the test timeout
	// is this a non-issue?
	ts.Config.ReadTimeout = DefaultReadTimeout
	ts.Config.WriteTimeout = DefaultWriteTimeout
	ts.Config.IdleTimeout = DefaultIdleTimeout
	ts.StartTLS()
	proxy := nettest.NewProxy(tb.Name(), strings.TrimPrefix(ts.URL, "https://"))
	tc := nettest.WithTransport(ts.Client(), "https://"+proxy.Listen())
	return &TestClient{tc, proxy, tb}
}

type TestUploader[V any] struct {
	Uploader[V]
}

func newTestUploader(tb testing.TB) *TestUploader[uuid.UUID] {
	url, err := compose.minio.ConnectionString(context.Background())
	is.OK(tb, err) // return minio connetion string

	var (
		bucket = ostest.Bucket(61) // random
		region = "auto"

		user = compose.minio.Username
		pass = compose.minio.Password
		cred = credentials.NewStaticCredentialsProvider(user, pass, "")
	)

	conf, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region), config.WithCredentialsProvider(cred))

	is.OK(tb, err) // return minio configuration

	// Create S3 service client
	// add toxiproxy
	client := s3.NewFromConfig(conf, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://" + url)
		o.UsePathStyle = true
	})

	// Create a new bucket using the CreateBucket call.
	// note: StatusCode: 409, BucketAlreadyOwnedByYou
	// note: StatusCode: 507, XMinioStorageFull
	p := &s3.CreateBucketInput{
		Bucket: &bucket,
	}
	_, err = client.CreateBucket(context.Background(), p)
	is.OK(tb, err) // create bucket

	return &TestUploader[uuid.UUID]{Uploader: ospkg.NewUploader(bucket, client)}
}

var compose struct {
	minio *minio.MinioContainer
}

func TestMain(m *testing.M) {
	err := setup(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	err = cleanup(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func setup(ctx context.Context) (err error) {
	compose.minio, err = minio.Run(ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z")
	if err != nil {
		return err
	}
	return
}

func cleanup(ctx context.Context) (err error) {
	var cc = []testcontainers.Container{compose.minio}
	for _, c := range cc {
		if c != nil {
			err = errors.Join(c.Terminate(ctx))
		}
	}
	return err
}
