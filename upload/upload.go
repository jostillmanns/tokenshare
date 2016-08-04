package main

import (
	"fmt"
	"log"
	"tokenshare"

	"honnef.co/go/js/dom"
)

var (
	butUpload  *dom.HTMLButtonElement
	divMessage *dom.HTMLDivElement

	openResult *tokenshare.OpenResult
	client     tokenshare.Client
)

func main() {
	d := dom.GetWindow().Document()
	butUpload = d.GetElementByID("upload").(*dom.HTMLButtonElement)
	divMessage = d.GetElementByID("message").(*dom.HTMLDivElement)
	openResult = &tokenshare.OpenResult{}

	butUpload.Disabled = true
	butUpload.Set("onclick", func() {
		go upload()
	})

	input := d.GetElementByID("file-input").(*dom.HTMLInputElement)
	input.AddEventListener("change", false, func(_ dom.Event) {
		go func() {
			client.Open(input, openResult)
			butUpload.Set("disabled", false)
		}()

	})

	client = tokenshare.Client{}

	pTok := d.GetElementByID("token").(*dom.HTMLParagraphElement)
	tok := token()
	pTok.SetInnerHTML(fmt.Sprintf("%v %s", tok.T, tok.Name)) // TODO Download
}

func message(m string) {
	divMessage.SetTextContent(m)
}

func token() tokenshare.Token {
	id, err := id()
	if err != nil {
		message("id not set")
		return tokenshare.Token{}
	}

	client := tokenshare.Client{}
	token, ok, err := client.Single(id)
	if err != nil {
		message(fmt.Sprintf("unable to query token: %v", err))
	}

	if !ok {
		message(fmt.Sprintf("%v", err))
	}

	return token
}

func upload() {
	res := openResult.Get()
	if res.Name == "" {
		log.Println("file not loaded")
		return
	}

	id, err := id()
	if err != nil {
		log.Println(err)
	}

	err = client.Upload(res.Data, res.Name, id, divMessage)
	if err != nil {
		message(err.Error())
	}
}

func id() (string, error) {
	u := (tokenshare.Client{}).Url()

	ids := u.Query()[tokenshare.ID]
	if len(ids) == 0 {
		return "", fmt.Errorf("id not set")
	}

	if ids[0] == "" {
		return "", fmt.Errorf("id not set")
	}

	return ids[0], nil
}
