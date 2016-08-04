package tokenshare

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"sync"

	"github.com/gopherjs/gopherjs/js"
	"honnef.co/go/js/dom"
)

type Client struct{}

func (c Client) Url() *url.URL {
	location := js.Global.Get("location").Get("href").String()

	u, _ := url.Parse(location)
	return u
}

func (c Client) List(div *dom.HTMLDivElement) error {
	d := dom.GetWindow().Document()

	toks, err := List(ReqList, nil)
	if err != nil {
		return err
	}

	if len(toks) == 0 {
		div.SetInnerHTML("no tokens yet")
		return nil
	}

	table := d.CreateElement("table").(*dom.HTMLTableElement)

	for i := range toks {
		row := d.CreateElement("tr").(*dom.HTMLTableRowElement)
		c.createRow(toks[i], row)

		table.AppendChild(row)
	}

	div.SetInnerHTML("")
	div.AppendChild(table)

	return nil
}

func (c Client) tokUrl(call string, tok Token) string {
	u := c.Url()
	form, _ := url.ParseQuery(u.RawQuery)
	form.Add(ID, hex.EncodeToString(tok.ID))
	u.RawQuery = form.Encode()
	u.Path = call

	return u.String()
}

func (c Client) tokDownloadUrl(tok Token) string {
	if tok.Name == "" {
		return ""
	}

	return fmt.Sprintf(`<a href="%s">%s</a>`, c.tokUrl("download", tok), tok.Name)
}

func (c Client) createRow(tok Token, row *dom.HTMLTableRowElement) {
	u := c.tokUrl("upload", tok)

	cell := row.InsertCell(0)
	cell.SetInnerHTML(hex.EncodeToString(tok.ID[:4]))

	cell = row.InsertCell(1)
	cell.SetInnerHTML(c.tokDownloadUrl(tok))

	cell = row.InsertCell(2)
	cell.SetInnerHTML(tok.T.String())

	cell = row.InsertCell(3)
	cell.SetInnerHTML(fmt.Sprintf(`<a href="%s">Upload Page</a>`, u))
}

func (c Client) Create(div *dom.HTMLDivElement) error {
	tok, err := Create(ReqCreate, nil)
	if err != nil {
		return err
	}

	table := div.ChildNodes()[0].(*dom.HTMLTableElement)
	d := dom.GetWindow().Document()
	row := d.CreateElement("tr").(*dom.HTMLTableRowElement)
	c.createRow(tok, row)
	table.AppendChild(row)

	return nil
}

func (c Client) Single(id string) (Token, bool, error) {
	m := make(map[string]string)
	m[ID] = id

	buf, err := Call(ReqSingle, nil, m)
	if err != nil {
		return Token{}, false, err
	}

	if string(buf) == (NoSuchToken{}).Error() {
		return Token{}, false, nil
	}

	token, err := Unmarshal(buf)
	if err != nil {
		return Token{}, false, err
	}

	return token, true, nil
}

type OpenResult struct {
	Data []byte
	Name string

	sync.Mutex
}

func (o *OpenResult) Set(data []byte, name string) {
	o.Lock()
	defer o.Unlock()

	o.Data = data
	o.Name = name
}

func (o *OpenResult) Get() *OpenResult {
	o.Lock()
	defer o.Unlock()

	return o
}

func (c Client) Open(input *dom.HTMLInputElement, res *OpenResult) {
	file := input.Get("files").Index(0)

	var wg sync.WaitGroup
	wg.Add(1)

	fileReader := js.Global.Get("FileReader").New()
	fileReader.Set("onload", c.open(res, file, fileReader, wg))
	fileReader.Call("readAsArrayBuffer", file)
	wg.Wait()
}

func (c Client) open(res *OpenResult, file, fileReader *js.Object, wg sync.WaitGroup) func() {
	return func() {
		defer wg.Done()
		name := file.Get("name").String()

		arrayBuffer := fileReader.Get("result")
		buf := js.Global.Get("Uint8Array").New(arrayBuffer).Interface().([]byte)

		res.Set(buf, name)
	}
}

func (c Client) Upload(p []byte, name, id string, progress *dom.HTMLDivElement) error {
	return c.upload(p, name, id, c.progress(len(p), progress))
}

func (c Client) progress(total int, div *dom.HTMLDivElement) func(int) {
	return func(p int) {
		if p == 0 {
			div.SetInnerHTML("Progress: 100%")
			return
		}

		fmt.Println(p)

		div.SetInnerHTML(fmt.Sprintf("Progress: %d%", int(float64(p)/float64(total)*100)))
	}
}

func (c Client) upload(data []byte, name, id string, progress func(int)) error {
	pr := make(chan int)
	defer close(pr)

	go func() {
		for i := range pr {
			progress(i)
		}
	}()

	return Transfer(ReqTransfer, name, id, data, pr)
}
