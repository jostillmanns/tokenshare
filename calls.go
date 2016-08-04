package tokenshare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
)

func MarshalList(t []Token) ([]byte, error) {
	return json.Marshal(t)
}

func Marshal(t Token) ([]byte, error) {
	return json.Marshal(t)
}

func Unmarshal(d []byte) (Token, error) {
	var t Token
	err := json.Unmarshal(d, &t)
	return t, err
}

func UnmarshalList(d []byte) ([]Token, error) {
	var t []Token
	err := json.Unmarshal(d, &t)
	return t, err
}

func Call(call string, cookie *http.Cookie, values map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", call, bytes.NewBuffer(nil))
	if err != nil {
		return nil, err
	}

	form, _ := url.ParseQuery(req.URL.RawQuery)
	for k, v := range values {
		form.Add(k, v)
	}
	req.URL.RawQuery = form.Encode()

	if cookie != nil {
		req.AddCookie(cookie)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request: %v", string(buf))
	}
	return buf, err
}

func Download(call, id string) ([]byte, error) {
	m := make(map[string]string)
	m[ID] = id

	return Call(call, nil, m)
}

func List(call string, cookie *http.Cookie) ([]Token, error) {
	buf, err := Call(call, cookie, make(map[string]string))
	if err != nil {
		return nil, err
	}

	return UnmarshalList(buf)
}

func Create(call string, cookie *http.Cookie) (Token, error) {
	buf, err := Call(call, cookie, make(map[string]string))
	if err != nil {
		return Token{}, err
	}

	var token Token
	if err := json.Unmarshal(buf, &token); err != nil {
		return Token{}, err
	}

	return token, nil
}

func Transfer(call, name, id string, data []byte, progress chan int) error {
	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(File, name)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(data)
	_, err = io.Copy(part, buf)
	if err != nil {
		return err
	}

	if err := writer.WriteField(ID, id); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	pr := &progressReader{body, progress}

	request, err := http.NewRequest("POST", call, pr)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return fmt.Errorf("upload: %s", string(buf))
	}

	return nil
}

type progressReader struct {
	r    io.Reader
	sent chan int
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if pr.sent != nil {
		pr.sent <- n
	}

	return n, err
}
