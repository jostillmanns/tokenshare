package main

import (
	"log"
	"tokenshare"

	"honnef.co/go/js/dom"
)

var (
	butCreate *dom.HTMLButtonElement
	divTokens *dom.HTMLDivElement
)

func main() {
	d := dom.GetWindow().Document()

	butCreate = d.GetElementByID("create").(*dom.HTMLButtonElement)
	divTokens = d.GetElementByID("tokens").(*dom.HTMLDivElement)

	var client tokenshare.Client

	butCreate.AddEventListener("click", false, func(event dom.Event) {
		go func() {
			if err := client.Create(divTokens); err != nil {
				log.Printf("create: %v", err)
			}
		}()
	})

	go func() {
		if err := client.List(divTokens); err != nil {
			log.Printf("list: %v", err)
		}
	}()
}
