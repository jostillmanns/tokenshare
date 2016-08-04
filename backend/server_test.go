package main

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"tokenshare"
)

func TestCreate(t *testing.T) {
	server, url, cookie, close := newTestServer(t)
	defer close()

	tok, err := tokenshare.Create(url+tokenshare.ReqCreate, cookie)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	buf, err := server.database.single(tok.ID)
	if err != nil {
		t.Fatalf("single: %v", err)
	}

	res, err := tokenshare.Unmarshal(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !bytes.Equal(res.ID, tok.ID) {
		t.Errorf("%v != %v", tok, res)
	}
}

func TestTransfer(t *testing.T) {
	server, url, _, close := newTestServer(t)
	defer close()

	tok, err := server.generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	id := hex.EncodeToString(tok.ID)
	name := "foobar"

	buf := []byte("GREETING")
	if err := tokenshare.Transfer(
		url+tokenshare.ReqTransfer,
		name,
		id,
		buf,
		nil,
	); err != nil {
		t.Fatalf("transfer: %v", err)
	}

	path := filepath.Join(server.storage, id, name)
	f, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("file read: %v", err)
	}

	if !bytes.Equal(f, buf) {
		t.Errorf("%s != %s", string(f), string(buf))
	}
}

func TestList(t *testing.T) {
	server, url, cookie, close := newTestServer(t)
	defer close()

	tok, err := server.generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err = server.database.single(tok.ID); err != nil {
		t.Fatalf("single: %v", err)
	}

	toks, err := tokenshare.List(url+tokenshare.ReqList, cookie)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(toks) < 1 {
		t.Fatalf("no toks found")
	}

	if !bytes.Equal(toks[0].ID, tok.ID) {
		t.Errorf("%v != %v", toks[0], tok)
	}
}

func TestDownload(t *testing.T) {
	server, url, _, close := newTestServer(t)
	defer close()

	tok, err := server.generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	buf := []byte("GREETING")
	dir := filepath.Join(server.storage, hex.EncodeToString(tok.ID))
	tok.Name = "foo"

	if err := server.database.update(tok.ID, tok); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(dir, tok.Name), buf, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	res, err := tokenshare.Download(url+tokenshare.ReqDownload, hex.EncodeToString(tok.ID))
	if err != nil {
		t.Fatalf("download: %v", err)
	}

	if !bytes.Equal(res, buf) {
		t.Errorf("%s != %s", string(res), string(buf))
	}
}

func newTestServer(t *testing.T) (*server, string, *http.Cookie, func()) {
	storage, err := ioutil.TempDir("", "tokenshare")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}

	bolt := filepath.Join(storage, "bolt")

	s, err := newServer(
		bolt,
		"token",
		storage,
		"www",
		"user",
		"pass",
		16,
		int64(20*20*1024),
	)
	if err != nil {
		t.Fatalf("server: %v", err)
	}

	server := httptest.NewServer(s.mux)

	req, err := http.NewRequest("GET", server.URL+"/index", bytes.NewBuffer([]byte{}))
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	req.SetBasicAuth("user", "pass")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("http body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index: %v", string(buf))
	}

	cookie := resp.Cookies()[0]
	if cookie == nil {
		t.Fatalf("missing cookie")
	}

	return s, server.URL, cookie, func() {
		os.RemoveAll(storage)
		server.Close()
	}
}
