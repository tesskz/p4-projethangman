package router

import (
    "net/http"

    "puissance4/controller"
)

func SetupRoutes(mux *http.ServeMux, app *controller.App) {
    mux.HandleFunc("/", app.Index)
    mux.HandleFunc("/about", app.About)
    mux.HandleFunc("/contact", app.Contact)
    mux.HandleFunc("/scoreboard", app.Scoreboard)

    mux.HandleFunc("/new", app.NewGame)
    mux.HandleFunc("/resume", app.Resume)
    mux.HandleFunc("/save", app.Save)
    mux.HandleFunc("/play", app.Play)
}
