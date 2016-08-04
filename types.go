package tokenshare

import "time"

type Token struct {
	ID   []byte    `json:id`
	T    time.Time `json:t`
	Name string    `jsoon:name`
}

const (
	ID   = "id"
	File = "file"

	ReqList     = "/list"
	ReqCreate   = "/create"
	ReqUpload   = "/upload"
	ReqSingle   = "/single"
	ReqTransfer = "/transfer"
	ReqDownload = "/download"
)

type NoSuchToken struct{}

func (_ NoSuchToken) Error() string {
	return "no such token"
}
