package publish

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/internal/lbrynet"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/ybbus/jsonrpc"
)

const uploadPath = "/tmp"

const fileField = "file"
const jsonrpcPayloadField = "json_payload"

type Publisher interface {
	Publish(string, string, []byte) ([]byte, error)
}

type LbrynetPublisher struct{}

type uploadHandler struct {
	Publisher  Publisher
	uploadPath string
}

func NewUploadHandler(uploadPath string, publisher Publisher) uploadHandler {
	return uploadHandler{
		Publisher:  publisher,
		uploadPath: uploadPath,
	}
}

func (p *LbrynetPublisher) Publish(filePath, accountID string, rawQuery []byte) ([]byte, error) {
	// var rpcParams *lbrynet.PublishParams
	// var rpcParams *ljsonrpc.StreamCreateOptions
	rpcParams := struct {
		Name                          string  `json:"name"`
		Bid                           string  `json:"bid"`
		FilePath                      string  `json:"file_path,omitempty"`
		FileSize                      *string `json:"file_size,omitempty"`
		IncludeProtoBuf               bool    `json:"include_protobuf"`
		Blocking                      bool    `json:"blocking"`
		*ljsonrpc.StreamCreateOptions `json:",flatten"`
	}{}

	query, err := proxy.NewQuery(rawQuery)
	if err != nil {
		panic(err)
	}

	if err := query.ParamsToStruct(&rpcParams); err != nil {
		panic(err)
	}

	if rpcParams.FilePath != "__POST_FILE__" {
		panic("unknown file_path content")
	}

	bid, err := strconv.ParseFloat(rpcParams.Bid, 64)
	rpcParams.FilePath = filePath
	rpcParams.AccountID = &accountID

	result, err := lbrynet.Client.StreamCreate(rpcParams.Name, filePath, bid, *rpcParams.StreamCreateOptions)
	if err != nil {
		return nil, err
	}

	rpcResponse := jsonrpc.RPCResponse{Result: result}
	serialized, err := json.MarshalIndent(rpcResponse, "", "  ")
	if err != nil {
		return nil, err
	}
	return serialized, nil
}

func (h uploadHandler) Handle(w http.ResponseWriter, r *users.AuthenticatedRequest) {
	if !r.IsAuthenticated() {
		var authErr Error
		if r.AuthFailed() {
			authErr = NewAuthError(r.AuthError)
		} else {
			authErr = ErrUnauthorized
		}
		w.WriteHeader(http.StatusOK)
		w.Write(authErr.AsBytes())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	f, err := h.CreateFile(r.AccountID, header.Filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	io.Copy(f, file)

	response, err := h.Publisher.Publish(f.Name(), r.AccountID, []byte(r.FormValue(jsonrpcPayloadField)))
	if err != nil {
		panic(err)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func (h uploadHandler) CreateFile(accountID string, origFilename string) (*os.File, error) {
	path, err := h.preparePath(accountID)
	if err != nil {
		panic(err)
	}
	return ioutil.TempFile(path, fmt.Sprintf("*_%v", origFilename))
}

func (h uploadHandler) preparePath(accountID string) (string, error) {
	path := path.Join(h.uploadPath, accountID)
	err := os.MkdirAll(path, os.ModePerm)
	return path, err
}
