package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"tokenshare"
)

func TestCreate(t *testing.T) {
	server, testSrv, cookie, close := newTestServer(t)
	defer close()

	tok, err := tokenshare.Create(testSrv.URL+tokenshare.ReqCreate, cookie)
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
	server, testSrv, _, close := newTestServer(t)
	defer close()

	tok, err := server.generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	id := hex.EncodeToString(tok.ID)
	name := "foobar"

	buf := make([]byte, 1024*1024*50)
	if err := tokenshare.Transfer(
		testSrv.URL+tokenshare.ReqTransfer,
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
	server, testSrv, cookie, close := newTestServer(t)
	defer close()

	tok, err := server.generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err = server.database.single(tok.ID); err != nil {
		t.Fatalf("single: %v", err)
	}

	toks, err := tokenshare.List(testSrv.URL+tokenshare.ReqList, cookie)
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
	server, testSrv, _, close := newTestServer(t)
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

	res, err := tokenshare.Download(testSrv.URL+tokenshare.ReqDownload, hex.EncodeToString(tok.ID))
	if err != nil {
		t.Fatalf("download: %v", err)
	}

	if !bytes.Equal(res, buf) {
		t.Errorf("%s != %s", string(res), string(buf))
	}
}

func TestTransferBrowser(t *testing.T) {
	once := sync.Once{}
	wg := sync.WaitGroup{}
	wg.Add(1)

	s, srv, _, clear := newTestServer(t)
	defer clear()

	bufSize := int64(1024 * 1024 * 50)

	s.mux.HandleFunc("/transfertest", func(w http.ResponseWriter, req *http.Request) {
		once.Do(func() {
			defer wg.Done()
			if err := req.ParseMultipartForm(s.maxMemory); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}

			file, _, err := req.FormFile(tokenshare.File)
			if err != nil {
				t.Fatalf("file open: %v", err)
			}

			buffer := bytes.NewBuffer(nil)
			n, err := io.Copy(buffer, file)
			if err != nil {
				t.Fatalf("copy: %v", err)
			}

			defer func() {
				_ = file.Close()
			}()

			if n != bufSize {
				t.Fatalf("%d != %d", 1024*1024*50, n)
			}
		})
	})

	srv.Close()
	srv = httptest.NewServer(s.mux)

	install := exec.Command("gopherjs", "install", "tokenshare/test")
	if err := install.Run(); err != nil {
		t.Fatalf("install: %v", err)
	}

	datadir, err := ioutil.TempDir("", "tokenshare-chrome")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	defer os.RemoveAll(datadir)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url parse: %v", err)
	}

	u.Path = "testing"
	browser := exec.Command("chromium", "--incognito", fmt.Sprintf("--user-data-dir=%s", datadir), u.String())
	if err := browser.Start(); err != nil {
		t.Fatalf("chrome: %v", err)
	}

	wg.Wait()
	if err := browser.Process.Kill(); err != nil {
		t.Fatalf("process kill: %v", err)
	}
}

func newTestServer(t *testing.T) (*server, *httptest.Server, *http.Cookie, func()) {
	storage, err := ioutil.TempDir("", "tokenshare")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}

	www, err := ioutil.TempDir("", "tokenshare-www")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}

	// copy necessary files
	for _, f := range []string{"index.html", "testing.html"} {
		src, err := os.Open(filepath.Join("www", f))
		if err != nil {
			t.Fatalf("open: %v", err)
		}

		dest, err := os.OpenFile(filepath.Join(www, f), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatalf("open: %v", err)
		}

		if _, err := io.Copy(dest, src); err != nil {
			t.Fatalf("copy: %v", err)
		}

		_ = src.Close()
		_ = dest.Close()
	}

	bolt := filepath.Join(storage, "bolt")

	s, err := newSrv(
		bolt,
		"token",
		storage,
		www,
		"user",
		"pass",
		16,
		int64(1024*1024*1024),
	)
	if err != nil {
		t.Fatalf("server: %v", err)
	}

	s.mux.HandleFunc("/testing", func(w http.ResponseWriter, req *http.Request) {
		s.file(w, www, "testing.html", "text/html; charset-utf-8")
	})

	h := func(call string) func(w http.ResponseWriter, req *http.Request) {
		return func(w http.ResponseWriter, req *http.Request) {
			path := os.Getenv("GOPATH")
			path = filepath.Join(path, "bin")
			s.file(w, path, call, "application/javascript")
		}
	}

	s.mux.HandleFunc("/testing.js", h("test.js"))
	s.mux.HandleFunc("/testing.js.map", h("test.js.map"))

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

	return s, server, cookie, func() {
		_ = os.RemoveAll(storage)
		_ = os.RemoveAll(www)
		server.Close()
	}
}
