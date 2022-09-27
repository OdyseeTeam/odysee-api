package blobs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/lbryio/lbry.go/v3/stream"
	"github.com/lbryio/reflector.go/db"
	"github.com/lbryio/reflector.go/reflector"
	"github.com/lbryio/reflector.go/store"
	pb "github.com/lbryio/types/v2/go"
)

const (
	// MaxChunkSize is the max size of decrypted blob.
	MaxChunkSize = stream.MaxBlobSize - 1

	// DefaultPrefetchLen is how many blobs we should prefetch ahead.
	// 3 should be enough to deliver 2 x 4 = 8MB/s streams.
	// however since we can't keep up, let's see if 2 works
	DefaultPrefetchLen = 2
)

type Source struct {
	filePath      string
	blobsPath     string
	finalPath     string
	stream        *pb.Stream
	blobsManifest []string
}

type Store struct {
	cfg map[string]string
	db  *db.SQL
}

type Uploader struct {
	uploader *reflector.Uploader
}

// NewSource initializes a blob splitter, takes source file and blobs destination path as arguments.
func NewSource(filePath, blobsPath string) (*Source, error) {
	s := Source{
		filePath:  filePath,
		blobsPath: blobsPath,
	}

	return &s, nil
}

// NewStore initializes blob storage with a config dictionary.
// Required parameters in the config map are MySQL DSN and S3 config for the reflector.
func NewStore(cfg map[string]string) (*Store, error) {
	db := &db.SQL{
		LogQueries: false,
	}
	err := db.Connect(cfg["databasedsn"])
	if err != nil {
		return nil, err
	}

	return &Store{
		cfg: cfg,
		db:  db,
	}, nil
}

// Uploader returns blob file uploader instance for the pre-configured store.
// Can only be used for one stream upload and discarded afterwards.
func (s *Store) Uploader() *Uploader {
	dbs := store.NewDBBackedStore(store.NewS3Store(
		s.cfg["key"], s.cfg["secret"], s.cfg["region"], s.cfg["bucket"],
	), s.db, false)
	return &Uploader{
		uploader: reflector.NewUploader(s.db, dbs, 5, false, false),
	}
}

func (s *Source) Split() (*pb.Stream, error) {
	file, err := os.Open(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open source: %w", err)
	}
	defer file.Close()

	enc := stream.NewEncoder(file)

	encodedStream, err := enc.Stream()
	if err != nil {
		return nil, fmt.Errorf("cannot create stream: %w", err)
	}
	s.stream = &pb.Stream{
		Source: &pb.Source{
			SdHash: enc.SDBlob().Hash(),
			Name:   filepath.Base(file.Name()),
			Size:   uint64(enc.SourceLen()),
			Hash:   enc.SourceHash(),
		},
	}

	s.finalPath = path.Join(s.blobsPath, enc.SDBlob().HashHex())
	err = os.MkdirAll(s.finalPath, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("cannot create directory for blobs: %w", err)
	}

	s.blobsManifest = make([]string, len(encodedStream))

	for i, b := range encodedStream {
		err := ioutil.WriteFile(path.Join(s.blobsPath, enc.SDBlob().HashHex(), b.HashHex()), b, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("cannot write blob: %w", err)
		}
		s.blobsManifest[i] = b.HashHex()
	}

	return s.stream, nil
}

func (s *Source) Stream() *pb.Stream {
	return s.stream
}

// Upload is a wrapper for uploading sreams to reflector.
// Split() should be called on the source first.
func (u *Uploader) Upload(source *Source) (*reflector.Summary, error) {
	if source.finalPath == "" || source.Stream() == nil {
		return nil, errors.New("source is not split to blobs")
	}
	err := u.uploader.Upload(source.finalPath)
	summary := u.uploader.GetSummary()
	if err != nil {
		return nil, fmt.Errorf("cannot upload blobs: %w", err)
	}
	return &summary, nil
}
