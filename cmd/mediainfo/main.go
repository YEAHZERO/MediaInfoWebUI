package main

import (
	"log"

	"mediainfo"
	"mediainfo/internal/app"
	"mediainfo/internal/version"
)

func main() {
	server, err := app.NewServer(mediainfo.EmbeddedWebUI())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("mediainfo %s starting...", version.Version)
	log.Printf("mediainfo listening on http://0.0.0.0%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
