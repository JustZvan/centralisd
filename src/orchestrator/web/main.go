package web

import (
	"fmt"
	"net/http"
)

func ServeWeb() {
	app := http.NewServeMux()

	app.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello")
	})

	http.ListenAndServe("localhost:8090", app)
}
