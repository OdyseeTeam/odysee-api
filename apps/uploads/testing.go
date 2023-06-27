package uploads

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/OdyseeTeam/odysee-api/apps/uploads/database"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/OdyseeTeam/odysee-api/pkg/queue"
	"github.com/Pallinder/go-randomdata"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type TestHelper struct {
	S3Config configng.S3Config
	DB       *sql.DB
	S3       *s3.S3
	Queries  *database.Queries
}

func NewTestHelper(t *testing.T) (*TestHelper, error) {
	th := &TestHelper{}
	s3cfg := configng.S3Config{
		Endpoint: "http://localhost:9002",
		Bucket:   fmt.Sprintf("test-uploads-%s-%s", randomdata.Noun(), randomdata.Adjective()),
		Region:   "us-east-1",
		Key:      "minio",
		Secret:   "minio123",
		Minio:    true,
	}
	th.S3Config = s3cfg

	client, err := configng.NewS3Client(s3cfg)
	if err != nil {
		return nil, err
	}

	t.Cleanup(func() {
		listObjectsOutput, err := client.ListObjects(&s3.ListObjectsInput{
			Bucket: aws.String(s3cfg.Bucket),
		})
		if err != nil {
			t.Logf("failed to list objects in bucket: %v", err)
			return
		}

		for _, object := range listObjectsOutput.Contents {
			t.Logf("deleting %s", *object.Key)
			_, err = client.DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(s3cfg.Bucket),
				Key:    object.Key,
			})
			if err != nil {
				t.Logf("failed to delete object: %v", err)
			}
		}

		_, err = client.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: aws.String(s3cfg.Bucket),
		})
		if err != nil {
			t.Logf("failed to delete bucket: %v", err)
		}
	})

	db, cleanup, err := migrator.CreateTestDB(migrator.DefaultDBConfig(), database.MigrationsFS)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { cleanup() })

	th.DB = db
	th.S3 = client
	th.Queries = database.New(db)
	return th, nil
}

func (th *TestHelper) CreateUpload(filePath string, queue *queue.Queue) (*database.Upload, error) {
	// This simulates IDs generated by TUS backend.
	uploadID := randomdata.RandStringRunes(32) + "+" + randomdata.RandStringRunes(32)
	uploadKey := randomdata.RandStringRunes(32)
	th.S3.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(th.S3Config.Bucket),
	})
	s, err := th.uploadFileToS3(filePath, uploadKey)
	if err != nil {
		return nil, err
	}
	up, err := th.Queries.CreateUpload(context.Background(), database.CreateUploadParams{
		UserID: int32(randomdata.Number(1, 10000)),
		ID:     uploadID,
		Size:   s,
	})
	if err != nil {
		return nil, err
	}
	// Simulate upload complete event
	handler := Handler{
		s3bucket: th.S3Config.Bucket,
		logger:   zapadapter.NewKV(nil),
		queries:  th.Queries,
		queue:    queue,
	}
	err = handler.completeUpload(database.MarkUploadCompletedParams{
		UserID:   up.UserID,
		ID:       up.ID,
		Filename: path.Base(filePath),
		Key:      uploadKey,
	})
	if err != nil {
		return nil, err
	}
	return &up, err
}

func (th *TestHelper) uploadFileToS3(filePath, key string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}
	fileSize := fileInfo.Size()

	fmt.Printf("uploading %s to %s/%s\n", filePath, th.S3Config.Bucket, key)
	_, err = th.S3.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(th.S3Config.Bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fileSize, err
	}
	return fileSize, nil
}
