package player

import (
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An MP4 file, size: 158433824 bytes, blobs: 77
const streamURL = "what#6769855a9aa43b67086f9ff3c1a5bacb5698a27a"

// An MP4 file, size: 128791189 bytes, blobs: 63
const knownSizeStreamURL = "known-size#0590f924bbee6627a2e79f7f2ff7dfb50bf2877c"

type knownStream struct {
	uri      string
	size     int64
	blobsNum int
}

var knownStreams = []knownStream{
	knownStream{uri: streamURL, size: 158433824, blobsNum: 77},
	knownStream{uri: knownSizeStreamURL, size: 128791189, blobsNum: 63},
}

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func TestNewPlayer(t *testing.T) {
	p := NewPlayer(&Opts{EnableLocalCache: true})
	assert.IsType(t, p.localCache, &fsCache{})
}

func TestPlayerResolveStream(t *testing.T) {
	p := NewPlayer(nil)
	s, err := p.ResolveStream("bolivians-flood-streets-protest-military-coup#389ba57c9f76b859c2763c4b9a419bd78b1a8dd0")
	require.NoError(t, err)
	err = p.RetrieveStream(s)
	require.NoError(t, err)
}

func TestPlayerResolveStreamNotFound(t *testing.T) {
	p := NewPlayer(nil)
	s, err := p.ResolveStream(randomString(20))
	assert.Equal(t, errStreamNotFound, err)
	assert.Nil(t, s)
}

func TestStreamSeek(t *testing.T) {
	p := NewPlayer(nil)

	for _, stream := range knownStreams {
		s, err := p.ResolveStream(stream.uri)
		require.NoError(t, err)
		err = p.RetrieveStream(s)
		require.NoError(t, err)

		// Seeking to the end
		n, err := s.Seek(0, io.SeekEnd)
		require.NoError(t, err)
		assert.EqualValues(t, stream.size, n)

		// Seeking to the middle of the stream
		n, err = s.Seek(stream.size/2, io.SeekStart)
		require.NoError(t, err)
		assert.EqualValues(t, stream.size/2, n)

		// Seeking back to the beginning of the stream
		n, err = s.Seek(-stream.size/2, io.SeekCurrent)
		require.NoError(t, err)
		assert.EqualValues(t, 0, n)

		n, err = s.Seek(0, io.SeekStart)
		require.NoError(t, err)
		assert.EqualValues(t, 0, n)

		s.Seek(0, io.SeekEnd)
		n, err = s.Seek(-999999999, io.SeekEnd)
		assert.EqualValues(t, 0, n)
		assert.Equal(t, errOutOfBounds, err)

		n, err = s.Seek(-99, io.SeekStart)
		assert.EqualValues(t, 0, n)
		assert.Equal(t, errSeekingBeforeStart, err)

		n, err = s.Seek(999999999, io.SeekStart)
		assert.EqualValues(t, 0, n)
		assert.Equal(t, errOutOfBounds, err)
	}
}

func TestBlobCalculator(t *testing.T) {
	type testInput struct {
		size, offset int64
		readLen      int
	}
	type testCase struct {
		testInput  testInput
		testOutput ChunkCalculator
	}

	// size: 128791189, has blobs: 62 + padding, last blob index: 61
	testCases := []testCase{
		testCase{testInput{158433824, 0, 512}, ChunkCalculator{158433824, 0, 0, 0, 0, 512, 0}},
		testCase{testInput{158433824, 2450019, 64000}, ChunkCalculator{158433824, 2450019, 1, 1, 352867 + 1, 64000, 352868}},
		testCase{testInput{128791189, 128791089, 99}, ChunkCalculator{128791189, 128791089, 61, 61, 864817 + 61, 99, 864878}},
		testCase{testInput{128791189, 0, 128791189}, ChunkCalculator{0, 128791189, 0, 61, 0, 864978, 0}},
		testCase{testInput{1e7, 2097149, 43}, ChunkCalculator{2097149, 43, 0, 1, 2097149, 41, 0}},
	}

	for n, row := range testCases {
		t.Run(fmt.Sprintf("row:%v", n), func(t *testing.T) {
			bc := NewChunkCalculator(row.testInput.size, row.testInput.offset, row.testInput.readLen)
			assert.Equal(t, row.testOutput.FirstChunkIdx, bc.FirstChunkIdx)
			assert.Equal(t, row.testOutput.LastChunkIdx, bc.LastChunkIdx)
			assert.Equal(t, row.testOutput.FirstChunkOffset, bc.FirstChunkOffset)
			assert.Equal(t, row.testOutput.LastChunkReadLen, bc.LastChunkReadLen)
			assert.Equal(t, row.testOutput.LastChunkOffset, bc.LastChunkOffset)
		})
	}
}

func TestStreamRead(t *testing.T) {
	p := NewPlayer(nil)
	s, err := p.ResolveStream(streamURL)
	require.NoError(t, err)

	err = p.RetrieveStream(s)
	require.NoError(t, err)

	n, err := s.Seek(4000000, io.SeekStart)
	require.NoError(t, err)
	require.EqualValues(t, 4000000, n)

	readData := make([]byte, 105)
	readNum, err := s.Read(readData)
	require.NoError(t, err)
	assert.Equal(t, 105, readNum)
	expectedData, err := hex.DecodeString(
		"6E81C93A90DD3A322190C8D608E29AA929867407596665097B5AE780412" +
			"61638A51C10BC26770AFFEF1533715FBD1428DCADEDC7BEA5D7A9C7D170" +
			"B71EF38E7138D24B0C7E86D791695EDAE1B88EDBE54F95C98EF3DCFD91D" +
			"A025C284EE37D8FEEA2EA84B76B9A22D3")
	require.NoError(t, err)
	assert.Equal(t, expectedData, readData)
}

func TestStreamReadHotCache(t *testing.T) {
	p := NewPlayer(&Opts{EnableLocalCache: true, EnablePrefetch: false})

	s, err := p.ResolveStream(streamURL)
	require.NoError(t, err)

	err = p.RetrieveStream(s)
	require.NoError(t, err)

	// Warm up the cache
	n, err := s.Seek(4000000, io.SeekStart)
	require.NoError(t, err)
	require.EqualValues(t, 4000000, n)

	readData := make([]byte, 105)
	readNum, err := s.Read(readData)
	require.NoError(t, err)
	assert.Equal(t, 105, readNum)

	///
	s, err = p.ResolveStream(streamURL)
	require.NoError(t, err)

	err = p.RetrieveStream(s)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		n, err := s.Seek(4000000, io.SeekStart)
		require.NoError(t, err)
		require.EqualValues(t, 4000000, n)

		readData := make([]byte, 105)
		readNum, err := s.Read(readData)
		require.NoError(t, err)
		assert.Equal(t, 105, readNum)
		expectedData, err := hex.DecodeString(
			"6E81C93A90DD3A322190C8D608E29AA929867407596665097B5AE780412" +
				"61638A51C10BC26770AFFEF1533715FBD1428DCADEDC7BEA5D7A9C7D170" +
				"B71EF38E7138D24B0C7E86D791695EDAE1B88EDBE54F95C98EF3DCFD91D" +
				"A025C284EE37D8FEEA2EA84B76B9A22D3")
		require.NoError(t, err)
		assert.Equal(t, expectedData, readData)
	}

	assert.IsType(t, &cachedChunk{}, s.chunkGetter.seenChunks[1])

	n, err = s.Seek(2000000, io.SeekCurrent)
	require.NoError(t, err)
	require.EqualValues(t, 6000105, n)

	readData = make([]byte, 105)
	readNum, err = s.Read(readData)
	require.NoError(t, err)
	assert.Equal(t, 105, readNum)
	require.NoError(t, err)

	assert.Nil(t, s.chunkGetter.seenChunks[1])
	assert.IsType(t, &reflectedChunk{}, s.chunkGetter.seenChunks[2])
}

func TestStreamReadOutOfBounds(t *testing.T) {
	p := NewPlayer(nil)
	s, err := p.ResolveStream(streamURL)
	require.NoError(t, err)

	err = p.RetrieveStream(s)
	require.NoError(t, err)

	n, err := s.Seek(4000000, io.SeekStart)
	require.NoError(t, err)
	require.EqualValues(t, 4000000, n)

	readData := make([]byte, 105)
	readNum, err := s.Read(readData)
	require.NoError(t, err)
	assert.Equal(t, 105, readNum)
	expectedData, err := hex.DecodeString(
		"6E81C93A90DD3A322190C8D608E29AA929867407596665097B5AE780412" +
			"61638A51C10BC26770AFFEF1533715FBD1428DCADEDC7BEA5D7A9C7D170" +
			"B71EF38E7138D24B0C7E86D791695EDAE1B88EDBE54F95C98EF3DCFD91D" +
			"A025C284EE37D8FEEA2EA84B76B9A22D3")
	require.NoError(t, err)
	assert.Equal(t, expectedData, readData)
}
