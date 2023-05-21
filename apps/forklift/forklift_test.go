package forklift

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/OdyseeTeam/odysee-api/apps/uploads"
	"github.com/OdyseeTeam/odysee-api/apps/uploads/database"
	"github.com/OdyseeTeam/odysee-api/internal/tasks"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/internal/testdeps"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/suite"
)

type forkliftSuite struct {
	suite.Suite
	helper   *ForkliftTestHelper
	upHelper *uploads.UploadTestHelper
	s3c      *s3.Client
}

func TestForkliftSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}
	suite.Run(t, new(forkliftSuite))
}

func (s *forkliftSuite) TestHandleTask() {
	redisHelper := testdeps.NewRedisTestHelper(s.T())

	retriever := NewS3Retriever(s.T().TempDir(), s.s3c)
	l := NewLauncher(
		WithReflectorConfig(s.helper.ReflectorConfig),
		WithBlobPath(s.T().TempDir()),
		WithRetriever(retriever),
		WithRedisURL(redisHelper.URL),
		WithLogger(zapadapter.NewKV(nil)),
		WithDB(s.upHelper.DB),
	)

	bus, err := l.Build()
	s.Require().NoError(err)

	merges := make(chan tasks.AsynqueryMergePayload)
	bus.AddHandler(tasks.TaskAsynqueryMerge, func(_ context.Context, task *asynq.Task) error {
		fmt.Println("got incoming task")
		var payload tasks.AsynqueryMergePayload
		err := json.Unmarshal(task.Payload(), &payload)
		s.Require().NoError(err)
		merges <- payload
		return nil
	})

	go bus.StartHandlers()
	defer func() {
		bus.Shutdown()
	}()

	cases := []struct {
		fileName string
		expected func(upload *database.Upload, payload tasks.AsynqueryMergePayload)
	}{
		{
			test.StaticAsset(s.T(), "image2.jpg"),
			func(upload *database.Upload, payload tasks.AsynqueryMergePayload) {
				s.Equal(upload.UserID, payload.UserID)
				s.Equal(upload.ID, payload.UploadID)
				s.Equal("image2.jpg", payload.Meta.FileName)
				s.Equal("image/jpeg", payload.Meta.MIME)
				s.EqualValues(375172, payload.Meta.Size)
				s.Equal(2048, payload.Meta.Width)
				s.Equal(1365, payload.Meta.Height)
				s.Empty(payload.Meta.Duration)
				s.NotEmpty(payload.Meta.SDHash)
				s.False(fileExists(s.s3c, s.upHelper.S3Config.Bucket, upload.Key))
			},
		},
		{
			test.StaticAsset(s.T(), "hdreel.mov"),
			func(upload *database.Upload, payload tasks.AsynqueryMergePayload) {
				s.Equal(upload.UserID, payload.UserID)
				s.Equal(upload.ID, payload.UploadID)
				s.Equal("hdreel.mov", payload.Meta.FileName)
				s.Equal("video/quicktime", payload.Meta.MIME)
				s.EqualValues(17809516, payload.Meta.Size)
				s.Equal(1920, payload.Meta.Width)
				s.Equal(1080, payload.Meta.Height)
				s.Equal(29, payload.Meta.Duration)
				s.NotEmpty(payload.Meta.SDHash)
				s.False(fileExists(s.s3c, s.upHelper.S3Config.Bucket, upload.Key))
			},
		},
		{
			test.StaticAsset(s.T(), "doc.pdf"),
			func(upload *database.Upload, payload tasks.AsynqueryMergePayload) {
				s.Equal(upload.UserID, payload.UserID)
				s.Equal(upload.ID, payload.UploadID)
				s.Equal("doc.pdf", payload.Meta.FileName)
				s.Equal("application/pdf", payload.Meta.MIME)
				s.EqualValues(474475, payload.Meta.Size)
				s.Empty(payload.Meta.Width)
				s.Empty(payload.Meta.Height)
				s.Empty(payload.Meta.Duration)
				s.NotEmpty(payload.Meta.SDHash)
				s.False(fileExists(s.s3c, s.upHelper.S3Config.Bucket, upload.Key))
			},
		},
	}

	for _, c := range cases {
		s.T().Logf("creating upload for %s", c.fileName)
		upload, err := s.upHelper.CreateUpload(c.fileName, bus.Client())
		s.Require().NoError(err)
		s.T().Logf("created upload for %s", c.fileName)

		payload := <-merges

		c.expected(upload, payload)
	}
}

func (s *forkliftSuite) SetupSuite() {
	flHelper := &ForkliftTestHelper{}
	err := flHelper.Setup(s.T())

	if err != nil {
		if errors.Is(err, ErrMissingEnv) {
			s.T().Skipf(err.Error())
		} else {
			s.FailNow(err.Error())
		}
	}

	upHelper, err := uploads.NewUploadTestHelper(s.T())
	s.Require().NoError(err)

	s3c, err := configng.NewS3ClientV2(upHelper.S3Config)
	s.Require().NoError(err)

	s.s3c = s3c
	s.helper = flHelper
	s.upHelper = upHelper
}

func putFileIntoBucket(client *s3.Client, bucket, key string, file *os.File) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	}
	_, err := client.PutObject(context.TODO(), input)
	return err
}

func deleteAllFilesInBucket(client *s3.Client, bucket string) error {
	listInput := &s3.ListObjectsV2Input{
		Bucket: &bucket,
	}
	resp, err := client.ListObjectsV2(context.TODO(), listInput)
	if err != nil {
		return fmt.Errorf("unable to list objects: %w", err)
	}

	var objects []types.ObjectIdentifier
	for _, object := range resp.Contents {
		objects = append(objects, types.ObjectIdentifier{
			Key: object.Key,
		})
	}

	deleteInput := &s3.DeleteObjectsInput{
		Bucket: &bucket,
		Delete: &types.Delete{
			Objects: objects,
		},
	}
	_, err = client.DeleteObjects(context.TODO(), deleteInput)
	if err != nil {
		return fmt.Errorf("unable to delete objects: %w", err)
	}

	return nil
}

func fileExists(client *s3.Client, bucket string, key string) bool {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := client.HeadObject(context.TODO(), input)
	if err != nil {
		var nfe *types.NoSuchKey
		if errors.As(err, &nfe) {
			return false
		}
		// Some other error occurred
		return false
	}

	return true
}
