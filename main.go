package main

import (
	"log"
	"net/http"
	"os"

	"puissance4/controller"
	"puissance4/router"
)

func main() {

	if err := os.MkdirAll("data", 0o755); err != nil {
		log.Fatalf("impossible de créer data/: %v", err)
	}

	app := controller.NewApp(
		"data/save.json",
		"data/scores.json",
		"template",
	)

	mux := http.NewServeMux()
	router.SetupRoutes(mux, app)

	addr := ":8080"
	log.Printf("Puissance 4 – serveur démarré sur http://localhost%v", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
