package tasks

import (
	"strconv"
)

// A list of task types.
const (
	TaskReflectUpload    = "forklift:process-upload"
	TaskProcessAsynquery = "asynquery:query"
	TaskAsynqueryMerge   = "asynquery:merge"
)

type AsyncQueryTask struct {
	ID     string
	UserID int `json:"user_id"`
}

type AsynqueryMergePayload struct {
	UploadID string `json:"upload_id"`
	UserID   int32  `json:"user_id"`
	Meta     UploadMeta
}

type ReflectUploadPayload struct {
	UploadID     string         `json:"upload_id"`
	FileName     string         `json:"file_name"`
	UserID       int32          `json:"user_id"`
	FileLocation FileLocationS3 `json:"file_location"`
}

type FileLocationS3 struct {
	Bucket string
	Key    string
}

type UploadMeta struct {
	Size      uint64
	FileName  string `json:"file_name"`
	SDHash    string `json:"sd_hash"`
	MIME      string
	Extension string
	Hash      string
	Duration  int `json:",omitempty"`
	Width     int `json:",omitempty"`
	Height    int `json:",omitempty"`
}

func (p AsynqueryMergePayload) GetTraceData() map[string]string {
	return map[string]string{
		"user_id":  strconv.Itoa(int(p.UserID)),
		"query_id": p.UploadID,
	}
}

func (p ReflectUploadPayload) GetTraceData() map[string]string {
	return map[string]string{
		"user_id":   strconv.Itoa(int(p.UserID)),
		"upload_id": p.UploadID,
	}
}
