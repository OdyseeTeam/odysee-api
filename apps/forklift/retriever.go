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

type Retriever interface {
	Retrieve(ctx context.Context, uploadID string, loc tasks.FileLocationS3) (*LocalFile, error)
}

type LocalFile struct {
	Name string
	Size int64
}

type s3Retriever struct {
	tempPath   string
	downloader *manager.Downloader
}

func NewS3Retriever(tempPath string, client *s3.Client) *s3Retriever {
	return &s3Retriever{
		tempPath:   tempPath,
		downloader: manager.NewDownloader(client),
	}
}

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
	// log.Debug("upload file retreived", "bucket", loc.Bucket, "key", loc.Key, "size", n)

	return &LocalFile{sf.Name(), n}, nil
}

func (f LocalFile) Cleanup() error {
	return os.RemoveAll(f.Name)
}
