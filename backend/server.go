package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"
	"tokenshare"

	"os"

	"encoding/hex"

	"github.com/boltdb/bolt"
)

const (
	index  = "index.html"
	upload = "upload.html"
	js     = "app.js"
)

func newSrv(db, bucket, storage, static, user, pass string, tokenSize int, maxMemory int64) (*server, error) {
	bolt, err := bolt.Open(db, 0600, nil)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	s := &server{
		user: user,
		pass: pass,

		mux:       mux,
		maxMemory: maxMemory,

		storage: storage,
		static:  static,

		database: database{
			db:      bolt,
			bucket:  bucket,
			tokSize: tokenSize,
		},
	}

	if err := s.init(); err != nil {
		return nil, err
	}

	mux.HandleFunc("/index.html", s.index)
	mux.HandleFunc("/index", s.index)
	mux.HandleFunc("/app.js", s.client)
	mux.HandleFunc("/app.js.map", s.client)
	mux.HandleFunc("/upload.js", s.client)
	mux.HandleFunc("/upload.js.map", s.client)
	mux.HandleFunc(tokenshare.ReqList, s.list)
	mux.HandleFunc(tokenshare.ReqDownload, s.download)
	mux.HandleFunc(tokenshare.ReqCreate, s.create)
	mux.HandleFunc(tokenshare.ReqUpload, s.upload)
	mux.HandleFunc(tokenshare.ReqTransfer, s.transfer)
	mux.HandleFunc(tokenshare.ReqSingle, s.single)

	return s, nil
}

type server struct {
	user, pass string
	mux        *http.ServeMux

	maxMemory int64

	storage string
	static  string

	database
}

func (s *server) write(name, id string, rdr io.Reader) error {
	dir := filepath.Join(s.storage, id)
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return err
	}

	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, rdr)
	return err
}

func (s *server) checkAuth(req *http.Request) bool {
	u, p, ok := req.BasicAuth()
	if !ok {
		return false
	}

	if u != s.user {
		return false
	}

	if p != s.pass {
		return false
	}

	return true
}

func (s *server) checkCookie(req *http.Request) bool {
	cookie, err := req.Cookie(s.user)
	if err != nil {
		return false
	}

	if cookie.Value != s.pass {
		return false
	}

	return true
}

func (s *server) index(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Tokenshare"`)
	if !s.checkCookie(req) {
		if !s.checkAuth(req) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	expiration := time.Now().Add(365 * 24 * time.Hour)
	cookie := &http.Cookie{Name: s.user, Value: s.pass, Expires: expiration}
	http.SetCookie(w, cookie)

	s.file(w, s.static, index, "text/html; charset-utf-8")
}

func (s *server) client(w http.ResponseWriter, req *http.Request) {
	path := os.Getenv("GOPATH")
	path = filepath.Join(path, "bin")

	s.file(w, path, req.URL.Path, "application/javascript")
}

func (s *server) list(w http.ResponseWriter, req *http.Request) {
	if !s.checkCookie(req) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	toks, err := s.database.list()
	if err != nil {
		http.Error(w, fmt.Sprintf("list: %v", err), http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(toks)
}

func (s *server) single(w http.ResponseWriter, req *http.Request) {
	id, err := hex.DecodeString(req.FormValue(tokenshare.ID))
	if err != nil {
		http.Error(w, fmt.Sprintf("hex decode: %v", err), http.StatusInternalServerError)
	}

	tok, err := s.database.single(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(tok)
}

func (s *server) create(w http.ResponseWriter, req *http.Request) {
	if !s.checkCookie(req) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tok, err := s.generate()
	if err != nil {
		http.Error(w, fmt.Sprintf("generate: %v", err), http.StatusInternalServerError)
		return
	}

	buf, err := tokenshare.Marshal(tok)
	if err != nil {
		http.Error(w, fmt.Sprintf("marhsal: %v", err), http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(buf)
}

func (s *server) file(w http.ResponseWriter, path, name string, contentType string) {
	file, err := ioutil.ReadFile(filepath.Join(path, name))
	if err != nil {
		http.Error(w, fmt.Sprintf("file: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(file)
}

func (s *server) upload(w http.ResponseWriter, req *http.Request) {
	s.file(w, s.static, upload, "text/html; charset-utf-8")
}
func (s *server) transfer(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(s.maxMemory); err != nil {
		http.Error(w, fmt.Sprintf("parse multipart form: %v", err), http.StatusBadGateway)
	}

	if req.MultipartForm == nil {
		http.Error(w, "no multipart form", http.StatusBadRequest)
		return
	}

	id := req.MultipartForm.Value[tokenshare.ID][0]
	bid, err := hex.DecodeString(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("hex decode: %v", err), http.StatusBadRequest)
	}

	token, ok, err := s.poke(bid)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to view database: %v", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, fmt.Sprintf("no such token: %s", id), http.StatusBadRequest)
	}

	file, handler, err := req.FormFile(tokenshare.File)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to open form file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if err := s.write(handler.Filename, id, file); err != nil {
		http.Error(w, fmt.Sprintf("unable to write file: %v", err), http.StatusInternalServerError)
		return
	}

	token.Name = handler.Filename

	if err := s.update(bid, token); err != nil {
		http.Error(w, fmt.Sprintf("unable to update token satus: %v", err), http.StatusInternalServerError)
		return
	}
}

func (s *server) download(w http.ResponseWriter, req *http.Request) {
	id := req.FormValue(tokenshare.ID)

	bid, err := hex.DecodeString(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("hex decode: %v", err), http.StatusBadRequest)
		return
	}

	tok, ok, err := s.database.poke(bid)
	if err != nil {
		http.Error(w, fmt.Sprintf("database: %v", err), http.StatusInternalServerError)
		return
	}

	if !ok {
		http.Error(w, fmt.Sprintf("no such token: %s", id), http.StatusBadRequest)
		return
	}

	w.Header().Set("content-disposition", fmt.Sprintf(`attachment; filename="%s"`, tok.Name))
	http.ServeFile(w, req, filepath.Join(s.storage, id, tok.Name))
}
