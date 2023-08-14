package forklift

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/tasks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	defaultRequestTimeout = 600 * time.Second
)

var errFatalRetrieverFailure = fmt.Errorf("fatal retriever error")

type LocalFile struct {
	Name string
	Size int64
}

type S3Retriever struct {
	tempPath   string
	downloader *manager.Downloader
	client     *s3.Client
}

type HTTPRetriever struct {
	tempPath string
	client   *http.Client
}

// NewS3Retriever creates a new retriever that downloads files from S3 uploads storage.
func NewS3Retriever(tempPath string, client *s3.Client) *S3Retriever {
	return &S3Retriever{
		tempPath:   tempPath,
		downloader: manager.NewDownloader(client),
		client:     client,
	}
}

// Retrieve downloads the uploaded file from S3 and returns a local file path.
func (r *S3Retriever) Retrieve(ctx context.Context, uploadID string, loc tasks.FileLocationS3) (*LocalFile, error) {
	sf, err := os.Create(path.Join(r.tempPath, uploadID))
	if err != nil {
		return nil, fmt.Errorf("failed to create local upload file (%s): %w", path.Join(r.tempPath, uploadID), err)
	}
	defer sf.Close()

	in := &s3.GetObjectInput{
		Bucket: aws.String(loc.Bucket),
		Key:    aws.String(loc.Key),
	}
	n, err := r.downloader.Download(ctx, sf, in)
	if err != nil {
		os.Remove(sf.Name())
		return nil, err
	}

	return &LocalFile{sf.Name(), n}, nil
}

// Delete removes the uploaded file and should be called after file processing is complete to the point
// of saving processing result to the database.
func (r *S3Retriever) Delete(ctx context.Context, loc tasks.FileLocationS3) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(loc.Bucket),
		Key:    aws.String(loc.Key),
	}
	_, err := r.client.DeleteObject(ctx, input)
	return err
}

func (f LocalFile) Cleanup() error {
	return os.Remove(f.Name)
}

// NewS3Retriever creates a new retriever that downloads files from S3 uploads storage.
func NewHTTPRetriever(tempPath string) *HTTPRetriever {
	return &HTTPRetriever{
		tempPath: tempPath,
		client: &http.Client{
			Timeout: defaultRequestTimeout,
		},
	}
}

// Retrieve downloads the uploaded file from S3 and returns a local file path.
func (r *HTTPRetriever) Retrieve(ctx context.Context, uploadID string, loc tasks.FileLocationHTTP) (*LocalFile, error) {
	httpReq, err := http.NewRequest(
		http.MethodGet,
		loc.URL,
		http.NoBody,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: error creating request", errFatalRetrieverFailure)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error fetching remote file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote server returned non-OK status: %v", resp.StatusCode)
	}
	defer resp.Body.Close()

	sf, err := os.Create(path.Join(r.tempPath, uploadID))
	if err != nil {
		return nil, fmt.Errorf("failed to create local file (%s): %w", path.Join(r.tempPath, uploadID), err)
	}
	defer sf.Close()
	n, err := io.Copy(sf, resp.Body)
	if err != nil {
		os.Remove(sf.Name())
		return nil, fmt.Errorf("error saving uploaded file: %w", err)
	}
	if n == 0 {
		sf.Close()
		os.Remove(sf.Name())
		return nil, errors.New("remote file is empty")
	}

	return &LocalFile{sf.Name(), n}, nil
}

func hashURL(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}
