package main

import (
	"log"
	"net/http"

	"github.com/atreya2011/slack-google-oauth2-test/internal/pkg/calsync"
	"github.com/atreya2011/slack-google-oauth2-test/internal/pkg/config"
)

func main() {
	// get token from yaml config file
	c := config.New()
	if err := c.Parse("config.yml"); err != nil {
		log.Fatalf("[ERROR] %s\n", err)
	}
	log.Println("loaded config file")
	// create new calsync instance
	var cs = calsync.New(c)
	// create server and listen
	http.HandleFunc("/", cs.HandleSlashCommand)
	http.HandleFunc("/callback", cs.HandleRedirect)
	http.Handle("/googleb6de904a41249ac0.html", http.HandlerFunc(handleGoogleVerification))
	cs.Log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleGoogleVerification(w http.ResponseWriter, r *http.Request) {
	log.Println("google site verification request received")
	http.ServeFile(w, r, "./googleb6de904a41249ac0.html")
}
