package asynquery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/responses"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"
)

const (
	maxMultipartMemory = 500 << 20 // 500 MB
	tidTimestampFormat = "2006-01-02T15:04"
)

var validClaimIDRegex = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)

type TranscodeUploadResponse struct {
	Status     string `json:"status"`
	ClaimID    string `json:"claim_id,omitempty"`
	TID        string `json:"tid,omitempty"`
	FilesCount int    `json:"files_count,omitempty"`
	Error      string `json:"error,omitempty"`
}

func GenerateTID(sdHash string, transcodedAt time.Time) string {
	h := sha256.New()
	h.Write([]byte(sdHash))
	return hex.EncodeToString(h.Sum([]byte(transcodedAt.Format(tidTimestampFormat))))
}

func getContentTypeForFile(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".m3u8":
		return "application/x-mpegURL"
	case ".ts":
		return "video/mp2t"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

func (h QueryHandler) UploadTranscodePackage(w http.ResponseWriter, r *http.Request) {
	responses.AddJSONContentType(w)

	user, err := auth.FromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(TranscodeUploadResponse{
			Status: "auth_error",
			Error:  err.Error(),
		})
		return
	}

	err = r.ParseMultipartForm(maxMultipartMemory)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(TranscodeUploadResponse{
			Status: "error",
			Error:  "failed to parse multipart form: " + err.Error(),
		})
		return
	}

	claimID := r.FormValue("claim_id")
	if !validClaimIDRegex.MatchString(claimID) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(TranscodeUploadResponse{
			Status: "error",
			Error:  "invalid claim_id: must be a 40-character hex string",
		})
		return
	}

	sdHash := r.FormValue("sd_hash")
	channelURL := r.FormValue("channel_url")
	claimURL := r.FormValue("claim_url")

	transcodedAt := time.Now().UTC()
	var tid string
	if sdHash != "" {
		tid = GenerateTID(sdHash, transcodedAt)
	} else {
		tid = claimID
	}

	// Prepare S3 client if configured
	var s3Client *s3.Client
	s3cfg := config.GetTranscoderS3Config()
	if s3cfg != nil {
		s3Client, err = configng.NewS3Client(*s3cfg)
		if err != nil {
			h.logger.Warn("cannot initialize transcoder S3 client", "err", err)
		}
	}

	filesProcessed := 0
	var totalSize int64

	for _, fileHeaders := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			safeFileName := filepath.Base(fileHeader.Filename)
			if safeFileName == "" || safeFileName == "." || safeFileName == ".." {
				continue
			}

			srcFile, err := fileHeader.Open()
			if err != nil {
				h.logger.Warn("failed to open uploaded file", "filename", safeFileName, "err", err)
				continue
			}

			fileBytes, err := io.ReadAll(srcFile)
			srcFile.Close()
			if err != nil {
				h.logger.Warn("failed to read uploaded file buffer", "filename", safeFileName, "err", err)
				continue
			}

			totalSize += int64(len(fileBytes))
			filesProcessed++

			if s3Client != nil && s3cfg != nil {
				contentType := getContentTypeForFile(safeFileName)
				s3Key := tid + "/" + safeFileName

				_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
					Bucket:      aws.String(s3cfg.Bucket),
					Key:         aws.String(s3Key),
					Body:        bytes.NewReader(fileBytes),
					ContentType: aws.String(contentType),
				})
				if err != nil {
					h.logger.Warn("failed to upload transcode fragment to S3", "key", s3Key, "bucket", s3cfg.Bucket, "err", err)
				}
			}
		}
	}

	// Register in the Transcoder DB (if configured)
	dbcfg := config.GetTranscoderDBConfig()
	if dbcfg != nil && sdHash != "" {
		go func() {
			db, err := sql.Open("postgres", dbcfg.DSN)
			if err != nil {
				h.logger.Warn("failed to open transcoder DB", "err", err)
				return
			}
			defer db.Close()

			manifestJSON, _ := json.Marshal(map[string]any{
				"url":           claimURL,
				"channel_url":   channelURL,
				"sd_hash":       sdHash,
				"tid":           tid,
				"transcoded_at": transcodedAt.Format(time.RFC3339),
			})

			storageName := "remote"
			insertQuery := `
				INSERT INTO videos (
					tid, sd_hash, url, released_at, channel, storage, path, size, manifest
				) VALUES (
					$1, $2, $3, NOW(), $4, $5, $6, $7, $8
				)
				ON CONFLICT (sd_hash) DO UPDATE SET
					updated_at = NOW(),
					tid = EXCLUDED.tid,
					path = EXCLUDED.path,
					size = EXCLUDED.size,
					manifest = EXCLUDED.manifest
			`
			_, err = db.Exec(insertQuery, tid, sdHash, claimURL, channelURL, storageName, tid, totalSize, string(manifestJSON))
			if err != nil {
				h.logger.Warn("failed to insert video record into transcoder DB", "err", err, "sd_hash", sdHash)
			} else {
				h.logger.Info("registered video record in transcoder DB", "tid", tid, "sd_hash", sdHash)
			}
		}()
	}

	h.logger.Info("transcode package processed", "user_id", user.ID, "claim_id", claimID, "tid", tid, "files_count", filesProcessed)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(TranscodeUploadResponse{
		Status:     "success",
		ClaimID:    claimID,
		TID:        tid,
		FilesCount: filesProcessed,
	})
}
