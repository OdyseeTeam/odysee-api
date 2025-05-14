package blobs

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sort"

	"github.com/lbryio/lbry.go/v3/stream"
	"github.com/lbryio/reflector.go/db"
	"github.com/lbryio/reflector.go/reflector"
	"github.com/lbryio/reflector.go/store"
	pb "github.com/lbryio/types/v2/go"
	"github.com/spf13/viper"
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
	filePath        string
	blobPath        string
	finalPath       string
	encodedFileName string
	stream          *pb.Stream
	blobsManifest   []string
}

type Store struct {
	db         *db.SQL
	mainStore  *store.DBBackedStore
	blobStores []store.BlobStore
	workers    int
}

type Uploader struct {
	uploader *reflector.Uploader
}

// NewSource initializes a blob splitter, takes source file and blobs destination path as arguments.
func NewSource(filePath, blobPath, encodedFileName string) *Source {
	s := Source{
		filePath:        filePath,
		blobPath:        blobPath,
		encodedFileName: encodedFileName,
	}
	return &s
}

func CreateStoresFromConfig(cfg *viper.Viper, path string) ([]store.BlobStore, error) {
	destinations := cfg.Sub(path)
	if destinations == nil {
		return nil, fmt.Errorf("empty config path: %s", path)
	}
	stores := []store.BlobStore{}

	keys := []string{}
	for key := range destinations.AllSettings() {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, subKey := range keys {
		subCfg := destinations.Sub(subKey)
		if subCfg == nil {
			return nil, fmt.Errorf("empty config path: %s.%s", path, subKey)
		}
		subCfg.Set("name", subKey)
		bs, err := store.S3StoreFactory(subCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 store from %s.%s: %w", path, subKey, err)
		}

		stores = append(stores, bs)
	}
	return stores, nil
}

// NewStore initializes blob storage with a config dictionary.
// Required parameters in the config map are MySQL DSN and S3 config for the reflector.
func NewStore(dsn string, destinations []store.BlobStore) (*Store, error) {
	db := &db.SQL{
		LogQueries: false,
	}

	err := db.Connect(dsn)
	if err != nil {
		return nil, err
	}

	st := store.NewDBBackedStore(store.DBBackedParams{
		Name: "global",
		Store: store.NewMultiWriterStore(store.MultiWriterParams{
			Name:         "s3",
			Destinations: destinations,
		}),
		DB:           db,
		DeleteOnMiss: false,
		MaxSize:      nil,
	})
	return &Store{
		db:         db,
		mainStore:  st,
		blobStores: destinations,
		workers:    1,
	}, nil
}

// SetWorkers sets the number of workers uploading each stream to the reflector.
func (s *Store) SetWorkers(workers int) {
	s.workers = workers
}

// Uploader returns blob file uploader instance for the pre-configured store.
// Can only be used for one stream upload and discarded afterwards.
func (s *Store) Uploader() *Uploader {
	return &Uploader{
		uploader: reflector.NewUploader(s.db, s.mainStore, s.workers, true, false),
	}
}

func (s *Store) BlobStores() []store.BlobStore {
	return s.blobStores
}

func (s *Source) Split() (*pb.Stream, error) {
	file, err := os.Open(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open source: %w", err)
	}
	defer file.Close()

	enc := stream.NewEncoder(file)
	enc.SetFilename(s.encodedFileName)

	s.finalPath = s.blobPath
	err = os.MkdirAll(s.finalPath, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("cannot create directory for blobs: %w", err)
	}

	s.blobsManifest, err = enc.Encode(func(h string, b []byte) error {
		return os.WriteFile(path.Join(s.finalPath, h), b, os.ModePerm)
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode stream: %w", err)
	}
	s.stream = &pb.Stream{
		Source: &pb.Source{
			SdHash: enc.SDBlob().Hash(),
			Name:   s.encodedFileName,
			Size:   uint64(enc.SourceLen()),
			Hash:   enc.SourceHash(),
		},
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
	fi, err := os.Stat(source.finalPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat source blobs: %w", err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("blob source %s is not a directory", source.finalPath)
	}
	err = u.uploader.Upload(source.finalPath)
	summary := u.uploader.GetSummary()
	if err != nil {
		return nil, fmt.Errorf("cannot upload blobs: %w", err)
	}
	return &summary, nil
}
