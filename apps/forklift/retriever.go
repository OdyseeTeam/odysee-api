package forklift

import (
	"context"
	"errors"
	"os"
	"path"

	"github.com/OdyseeTeam/odysee-api/internal/tasks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type LocalFile struct {
	Name string
	Size int64
}

type s3Retriever struct {
	tempPath   string
	downloader *manager.Downloader
	client     *s3.Client
}

// NewS3Retriever creates a new retriever that downloads files from S3 uploads storage.
func NewS3Retriever(tempPath string, client *s3.Client) *s3Retriever {
	return &s3Retriever{
		tempPath:   tempPath,
		downloader: manager.NewDownloader(client),
		client:     client,
	}
}

// Retrieve downloads the uploaded file from S3 and returns a local file path.
func (r *s3Retriever) Retrieve(ctx context.Context, uploadID string, loc tasks.FileLocationS3) (*LocalFile, error) {
	sf, err := os.Create(path.Join(r.tempPath, uploadID))
	if err != nil {
		return nil, errors.New("failed to create local upload file")
	}
	defer sf.Close()

	in := &s3.GetObjectInput{
		Bucket: aws.String(loc.Bucket),
		Key:    aws.String(loc.Key),
	}
	n, err := r.downloader.Download(context.TODO(), sf, in)
	if err != nil {
		return nil, err
	}

	return &LocalFile{sf.Name(), n}, nil
}

// Delete removes the uploaded file and should be called after file processing is complete to the point
// of saving processing result to the database.
func (r *s3Retriever) Delete(ctx context.Context, loc tasks.FileLocationS3) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(loc.Bucket),
		Key:    aws.String(loc.Key),
	}
	_, err := r.client.DeleteObject(context.TODO(), input)
	return err
}

func (f LocalFile) Cleanup() error {
	return os.RemoveAll(f.Name)
}
