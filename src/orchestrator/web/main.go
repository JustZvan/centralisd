package web

import (
	"html/template"
	"net/http"
	"strings"
)

// TODO: Bundle templates with the app

type viewData struct {
	Title           string
	ContentTemplate string

	NodesOpen     bool
	Clusters      []string
	ActiveCluster string
}

var hardcodedClusters = []string{
	"nl-amsterdam-1",
	"de-frankfurt-1",
} // fix asap

func isKnownCluster(id string) bool {
	for _, c := range hardcodedClusters {
		if c == id {
			return true
		}
	}
	return false
}

func parseTemplates() (*template.Template, error) {
	return template.ParseGlob("templates/*.html")
}

func ServeWeb() {
	app := http.NewServeMux()

	tmpl := template.Must(parseTemplates())

	static := http.FileServer(http.Dir("static"))
	app.Handle("/static/", http.StripPrefix("/static/", static))

	render := func(w http.ResponseWriter, data viewData) {
		if data.Clusters == nil {
			data.Clusters = hardcodedClusters
		}
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			http.Error(w, "template render error", http.StatusInternalServerError)
		}
	}

	app.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		render(w, viewData{
			Title:           "Centralis",
			ContentTemplate: "page_home",
			NodesOpen:       false,
		})
	})

	app.HandleFunc("/clusters/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/clusters/")
		id = strings.Trim(id, "/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		if !isKnownCluster(id) {
			http.NotFound(w, r)
			return
		}

		render(w, viewData{
			Title:           "Cluster: " + id,
			ContentTemplate: "page_cluster",
			NodesOpen:       true,
			ActiveCluster:   id,
		})
	})

	http.ListenAndServe("localhost:8090", app)
}
