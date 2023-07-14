package publish

import (
	"os"
	"path"
	"path/filepath"

	"github.com/lbryio/lbry.go/v3/stream"
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

func Streamize(p string) (stream.Stream, *pb.Stream, error) {
	file, err := os.Open(p)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	enc := stream.NewEncoder(file)

	s, err := enc.Stream()
	if err != nil {
		return nil, nil, err
	}
	streamProto := &pb.Stream{
		Source: &pb.Source{
			SdHash: enc.SDBlob().Hash(),
			Name:   filepath.Base(file.Name()),
			Size:   uint64(enc.SourceLen()),
			Hash:   enc.SourceHash(),
		},
	}

	err = os.Mkdir(enc.SDBlob().HashHex(), os.ModePerm)
	if err != nil {
		return nil, nil, err
	}

	for _, b := range s {
		os.WriteFile(path.Join(enc.SDBlob().HashHex(), b.HashHex()), b, os.ModePerm)
	}

	return s, streamProto, nil
}
