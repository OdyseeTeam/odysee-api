package upload

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	mrand "math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/apps/upload/database"
	"github.com/OdyseeTeam/odysee-api/internal/e2etest"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"
	"github.com/go-chi/chi/v5"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/suite"
	"github.com/tus/tusd/pkg/s3store"
)

type uploadSuite struct {
	suite.Suite

	launcher *Launcher
	keyfob   *keybox.Keyfob
	router   chi.Router
	db       *sql.DB
	cleanups []func()
}

func (s *uploadSuite) TestUpload() {
	testServer := httptest.NewServer(s.router)
	defer testServer.Close()

	queries := database.New(s.db)

	fnb64 := base64.StdEncoding.EncodeToString([]byte("dummy.md"))
	f := []byte("test file")

	userID := int64(randomdata.Number(1, 1000000))
	baseURL := "/v1/uploads/"
	token, err := s.keyfob.GenerateToken(userID, time.Now().Add(time.Hour*24))
	s.Require().NoError(err)
	tokenHeader := fmt.Sprintf("Bearer %s", token)

	(&test.HTTPTest{
		Method:  http.MethodGet,
		URL:     testServer.URL + "/healthz",
		ResBody: "OK",
		Code:    http.StatusOK,
	}).RunHTTP(s.T())

	(&test.HTTPTest{
		Method:  http.MethodGet,
		URL:     testServer.URL + "/livez",
		ResBody: "OK",
		Code:    http.StatusOK,
	}).RunHTTP(s.T())

	response := (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    testServer.URL + baseURL,
		ReqHeader: map[string]string{
			"Tus-Resumable":   "1.0.0",
			"Upload-Length":   fmt.Sprintf("%d", len(f)),
			"Upload-Metadata": fmt.Sprintf("filename %s", fnb64),
			// "Upload-Offset":            "0",
			// "Content-Type":             "application/offset+octet-stream",
			AuthorizationHeader: tokenHeader,
		},
		ReqBody: bytes.NewReader(f),
		Code:    http.StatusCreated,
	}).RunHTTP(s.T())
	loc, err := url.Parse(response.Header.Get("Location"))
	s.Require().NoError(err)
	s.Require().Regexp(baseURL+"[a-z0-9]{32}", loc.RequestURI())

	uploadID := filepath.Base(loc.Path)
	var upload database.Upload

	e2etest.Wait(s.T(), "upload settling into database", 5*time.Second, 1000*time.Millisecond, func() error {
		var err error
		upload, err = queries.GetUpload(context.Background(), database.GetUploadParams{UserID: strconv.FormatInt(userID, 10), ID: uploadID})
		if errors.Is(err, sql.ErrNoRows) {
			return e2etest.ErrWaitContinue
		} else if err != nil {
			return err
		}
		return nil
	})
	s.Equal(database.UploadStatusCreated, upload.Status)

	(&test.HTTPTest{
		Method: http.MethodPatch,
		URL:    testServer.URL + loc.RequestURI(),
		ReqHeader: map[string]string{
			"Tus-Resumable": "1.0.0",
			// "Upload-Length":            fmt.Sprintf("%d", len(f)),
			"Upload-Offset":     "0",
			"Content-Type":      "application/offset+octet-stream",
			AuthorizationHeader: tokenHeader,
		},
		ReqBody: bytes.NewReader(f),
		Code:    http.StatusNoContent,
	}).RunHTTP(s.T())

	e2etest.Wait(s.T(), "upload settling into database", 5*time.Second, 100*time.Millisecond, func() error {
		var err error
		upload, err = queries.GetUpload(context.Background(), database.GetUploadParams{UserID: strconv.FormatInt(userID, 10), ID: uploadID})
		if errors.Is(err, sql.ErrNoRows) {
			return e2etest.ErrWaitContinue
		} else if err != nil {
			return err
		} else if upload.Status != database.UploadStatusCompleted {
			return e2etest.ErrWaitContinue
		}
		return nil
	})

	s.Equal("dummy.md", upload.Filename)
}

func (s *uploadSuite) TestUploadLarger() {
	testServer := httptest.NewServer(s.router)
	defer testServer.Close()

	queries := database.New(s.db)

	var fileSize uint64 = 1024 * 1024 * 10
	var chunkSize uint64 = 1024 * 1024 * 2
	var uploadID string

	baseURL := "/v1/uploads/"
	userID := int64(randomdata.Number(1, 1000000))
	token, err := s.keyfob.GenerateToken(userID, time.Now().Add(time.Hour*24))
	tokenHeader := fmt.Sprintf("Bearer %s", token)
	s.Require().NoError(err)

	file := s.createRandomFile(fileSize)
	defer file.Close()

	tusUploadURL := testServer.URL + baseURL
	response := (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    tusUploadURL,
		Code:   http.StatusCreated,
		ReqHeader: map[string]string{
			AuthorizationHeader: tokenHeader,
			"Tus-Resumable":     "1.0.0",
			"Upload-Metadata":   fmt.Sprintf("filename %s", base64.StdEncoding.EncodeToString([]byte(file.Name()))),
			"Upload-Length":     fmt.Sprintf("%d", fileSize),
		},
	}).RunHTTP(s.T())

	loc, err := url.Parse(response.Header.Get("Location"))

	s.Require().NoError(err)
	s.Require().Regexp(baseURL+"[a-z0-9]{32}", loc.RequestURI())
	uploadID = filepath.Base(loc.Path)
	tusUploadURL = testServer.URL + loc.RequestURI()

	var upload database.Upload

	e2etest.Wait(s.T(), "upload settling into database", 5*time.Second, 1000*time.Millisecond, func() error {
		var err error
		upload, err = queries.GetUpload(context.Background(), database.GetUploadParams{UserID: strconv.FormatInt(userID, 10), ID: uploadID})
		if errors.Is(err, sql.ErrNoRows) {
			return e2etest.ErrWaitContinue
		} else if err != nil {
			return err
		}
		return nil
	})
	s.Equal(database.UploadStatusCreated, upload.Status)

	for i := uint64(0); i < fileSize; i += chunkSize {
		end := i + chunkSize
		if end > fileSize {
			end = fileSize
		}

		chunk := make([]byte, end-i)
		_, err := file.Read(chunk)
		s.Require().NoError(err)

		(&test.HTTPTest{
			Method: http.MethodPatch,
			URL:    tusUploadURL,
			Code:   http.StatusNoContent,
			ReqHeader: map[string]string{
				AuthorizationHeader: tokenHeader,
				"Tus-Resumable":     "1.0.0",
				"Upload-Offset":     fmt.Sprintf("%d", i),
				"Content-Type":      "application/offset+octet-stream",
			},
			ReqBody: bytes.NewReader(chunk),
		}).RunHTTP(s.T())
	}

	e2etest.Wait(s.T(), "upload settling into database", 5*time.Second, 100*time.Millisecond, func() error {
		var err error
		upload, err = queries.GetUpload(context.Background(), database.GetUploadParams{UserID: strconv.FormatInt(userID, 10), ID: uploadID})
		if errors.Is(err, sql.ErrNoRows) {
			return e2etest.ErrWaitContinue
		} else if err != nil {
			return err
		} else if upload.Status != database.UploadStatusCompleted {
			return e2etest.ErrWaitContinue
		}
		return nil
	})

	s.Equal(file.Name(), upload.Filename)
}

func (s *uploadSuite) TestUploadWrongToken() {
	testServer := httptest.NewServer(s.router)
	defer testServer.Close()

	queries := database.New(s.db)

	var fileSize uint64 = 1024 * 1024 * 10
	var chunkSize uint64 = 1024 * 1024 * 2
	var uploadID string

	baseURL := "/v1/uploads/"
	userID := int64(randomdata.Number(1, 1000000))
	token, err := s.keyfob.GenerateToken(userID, time.Now().Add(24*time.Hour))
	s.Require().NoError(err)
	tokenHeader := fmt.Sprintf("Bearer %s", token)

	wrongTokenGens := []struct {
		code int
		gen  func() string
	}{
		{
			http.StatusNotFound,
			func() string {
				token, err := s.keyfob.GenerateToken(int64(randomdata.Number(1000000, 9000000)), time.Now().Add(time.Hour*24))
				s.Require().NoError(err)
				return fmt.Sprintf("Bearer %s", token)
			},
		},
		{
			http.StatusUnauthorized,
			func() string {
				token, err := s.keyfob.GenerateToken(userID, time.Now().Add(-24*time.Hour))
				s.Require().NoError(err)
				return fmt.Sprintf("Bearer %s", token)
			}},
		{
			http.StatusUnauthorized,
			func() string {
				return "Bearer ZZZ"
			},
		},
		{
			http.StatusUnauthorized,
			func() string {
				return ""
			},
		},
	}

	file := s.createRandomFile(fileSize)
	defer file.Close()

	tusUploadURL := testServer.URL + baseURL

	wtg := wrongTokenGens[mrand.Intn(len(wrongTokenGens))]

	// Try a wrong token for upload creation first.
	(&test.HTTPTest{
		Method: http.MethodPost,
		URL:    tusUploadURL,
		Code:   wtg.code,
		ReqHeader: map[string]string{
			AuthorizationHeader: wtg.gen(),
			"Tus-Resumable":     "1.0.0",
			"Upload-Metadata":   fmt.Sprintf("filename %s", base64.StdEncoding.EncodeToString([]byte(file.Name()))),
			"Upload-Length":     fmt.Sprintf("%d", fileSize),
		},
	}).RunHTTP(s.T())

	response := (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    tusUploadURL,
		Code:   http.StatusCreated,
		ReqHeader: map[string]string{
			AuthorizationHeader: tokenHeader,
			"Tus-Resumable":     "1.0.0",
			"Upload-Metadata":   fmt.Sprintf("filename %s", base64.StdEncoding.EncodeToString([]byte(file.Name()))),
			"Upload-Length":     fmt.Sprintf("%d", fileSize),
		},
	}).RunHTTP(s.T())

	loc, err := url.Parse(response.Header.Get("Location"))

	s.Require().NoError(err)
	s.Require().Regexp(baseURL+"[a-z0-9]{32}", loc.RequestURI())
	uploadID = filepath.Base(loc.Path)
	tusUploadURL = testServer.URL + loc.RequestURI()

	var upload database.Upload

	e2etest.Wait(s.T(), "upload settling into database", 5*time.Second, 1000*time.Millisecond, func() error {
		var err error
		upload, err = queries.GetUpload(context.Background(), database.GetUploadParams{UserID: strconv.FormatInt(userID, 10), ID: uploadID})
		if errors.Is(err, sql.ErrNoRows) {
			return e2etest.ErrWaitContinue
		} else if err != nil {
			return err
		}
		return nil
	})
	s.Equal(database.UploadStatusCreated, upload.Status)

	for i := uint64(0); i < fileSize; i += chunkSize {
		wtg := wrongTokenGens[mrand.Intn(len(wrongTokenGens))]

		end := i + chunkSize
		if end > fileSize {
			end = fileSize
		}

		chunk := make([]byte, end-i)
		_, err := file.Read(chunk)
		s.Require().NoError(err)

		(&test.HTTPTest{
			Method: http.MethodPatch,
			URL:    tusUploadURL,
			Code:   wtg.code,
			ReqHeader: map[string]string{
				AuthorizationHeader: wtg.gen(),
				"Tus-Resumable":     "1.0.0",
				"Upload-Offset":     fmt.Sprintf("%d", i),
				"Content-Type":      "application/offset+octet-stream",
			},
			ReqBody: bytes.NewReader(chunk),
		}).RunHTTP(s.T())
	}
}

func (s *uploadSuite) SetupSuite() {
	config.Config.Override("PublishSourceDir", s.T().TempDir())
	config.Config.Override("GeoPublishSourceDir", s.T().TempDir())

	bucket := fmt.Sprintf("test-uploads-%s-%s", randomdata.Noun(), randomdata.Adjective())
	sess, err := session.NewSession(
		aws.NewConfig().
			WithS3ForcePathStyle(true). // needed for minio
			WithCredentials(credentials.NewStaticCredentials("minio", "minio123", "")).
			WithEndpoint("http://localhost:9000").
			WithRegion("us-east-1"),
	)
	s.Require().NoError(err)
	client := s3.New(sess)
	store := s3store.New(bucket, client)
	_, err = client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); !ok || awsErr.Code() != "BucketAlreadyOwnedByYou" {
			s.FailNow(err.Error())
		}
	}
	s.cleanups = append(s.cleanups, func() {
		listObjectsOutput, err := client.ListObjects(&s3.ListObjectsInput{
			Bucket: aws.String(bucket),
		})
		if err != nil {
			s.T().Logf("failed to list objects in bucket: %v", err)
			return
		}

		for _, object := range listObjectsOutput.Contents {
			s.T().Logf("deleting %s", *object.Key)
			_, err = client.DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    object.Key,
			})
			if err != nil {
				s.T().Logf("failed to delete object: %v", err)
			}
		}

		_, err = client.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
		if err != nil {
			s.T().Logf("failed to delete bucket: %v", err)
		}
	})

	redisOpts, err := redis.ParseURL("redis://:odyredis@localhost:6379/0")
	if err != nil {
		panic(fmt.Errorf("cannot parse redis config: %w", err))
	}
	locker, err := redislocker.New(redisOpts)
	if err != nil {
		panic(fmt.Errorf("cannot start redislocker: %w", err))
	}
	s.T().Cleanup(func() {
		redis.NewClient(redisOpts).FlushDB(context.Background())
	})

	kf, err := keybox.GenerateKeyfob()
	s.Require().NoError(err)

	db, cleanup, err := migrator.CreateTestDB(migrator.DefaultDBConfig(), database.MigrationsFS)
	s.Require().NoError(err)
	s.T().Cleanup(func() { cleanup() })

	l := NewLauncher().FileLocker(locker).Store(store).DB(db).PublicKey(kf.PublicKey()).Logger(zapadapter.NewKV(nil))
	r, err := l.Build()
	s.Require().NoError(err)

	s.router = r
	s.db = db
	s.launcher = l
	s.keyfob = kf
}

func (s *uploadSuite) TearDownSuite() {
}

func (s *uploadSuite) createRandomFile(fileSize uint64) *os.File {
	f, err := os.CreateTemp(s.T().TempDir(), "random-file-*.bin")
	s.Require().NoError(err)

	data := make([]byte, fileSize)
	_, err = rand.Read(data)
	s.Require().NoError(err)

	_, err = f.Write(data)
	s.Require().NoError(err)

	f, err = os.Open(f.Name())
	s.Require().NoError(err)
	return f
}

func TestUploadSuite(t *testing.T) {
	suite.Run(t, new(uploadSuite))
}
