package web

import (
	"centralisd/src/core/protocol"
	"centralisd/src/orchestrator/registry"
	"centralisd/src/orchestrator/tcp"
	ctmpl "centralisd/templates"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

// TODO: Bundle templates with the app

type viewData struct {
	Title             string
	ActiveClusterItem string

	NodesOpen     bool
	Clusters      []string
	ActiveCluster string
	ClusterOnline bool

	Nodes     []registry.NodeInfo
	VMDomains []protocol.VMListNode
}

type templates struct {
	home         *template.Template
	cluster      *template.Template
	vms          *template.Template
	reverseProxy *template.Template
	nodes        *template.Template
}

func parseTemplates() (*templates, error) {
	base := []string{
		"layout.html",
		"root.html",
		"partials_nav.html",
		"partials_sidebar_nodes.html",
	}

	parsePage := func(pageFile string) (*template.Template, error) {
		files := append(append([]string{}, base...), pageFile)
		return template.New("").ParseFS(ctmpl.FS, files...)
	}

	home, err := parsePage("page_home.html")
	if err != nil {
		return nil, err
	}
	cluster, err := parsePage("page_cluster.html")
	if err != nil {
		return nil, err
	}
	vms, err := parsePage("page_vms.html")
	if err != nil {
		return nil, err
	}
	reverseProxy, err := parsePage("page_reverse_proxy.html")
	if err != nil {
		return nil, err
	}
	nodes, err := parsePage("page_nodes.html")
	if err != nil {
		return nil, err
	}

	return &templates{
		home:         home,
		cluster:      cluster,
		vms:          vms,
		reverseProxy: reverseProxy,
		nodes:        nodes,
	}, nil
}

func ServeWeb(store *registry.Store, listenAddr string) {
	app := http.NewServeMux()

	tmpls, err := parseTemplates()
	if err != nil {
		panic(err)
	}

	static := http.FileServer(http.Dir("static"))
	app.Handle("/static/", http.StripPrefix("/static/", static))

	render := func(w http.ResponseWriter, tmpl *template.Template, data viewData) {
		data.Clusters = store.ActiveClusters()
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			http.Error(w, "template render error", http.StatusInternalServerError)
		}
	}

	app.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		render(w, tmpls.home, viewData{
			Title:     "Centralis",
			NodesOpen: false,
		})
	})

	app.HandleFunc("/clusters/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/clusters/")
		rest = strings.Trim(rest, "/")
		if rest == "" {
			http.NotFound(w, r)
			return
		}

		parts := strings.Split(rest, "/")
		if len(parts) < 1 || len(parts) > 2 {
			http.NotFound(w, r)
			return
		}
		id := parts[0]
		if id == "" {
			http.NotFound(w, r)
			return
		}
		if !store.IsKnownCluster(id) {
			http.NotFound(w, r)
			return
		}

		clusterItem := ""
		title := "Cluster: " + id
		tmpl := tmpls.cluster
		if len(parts) == 2 {
			switch parts[1] {
			case "vms":
				clusterItem = "vms"
				tmpl = tmpls.vms
				title = "VMs: " + id
			case "reverse-proxy":
				clusterItem = "reverse-proxy"
				tmpl = tmpls.reverseProxy
				title = "Reverse Proxy: " + id
			case "nodes":
				clusterItem = "nodes"
				tmpl = tmpls.nodes
				title = "Nodes: " + id
			default:
				http.NotFound(w, r)
				return
			}
		}

		nodes := store.NodesForCluster(id)

		data := viewData{
			Title:             title,
			NodesOpen:         true,
			ActiveCluster:     id,
			ActiveClusterItem: clusterItem,
			ClusterOnline:     store.IsClusterOnline(id),
			Nodes:             nodes,
		}

		if clusterItem == "vms" {
			data.VMDomains = fetchVMDomains(store, id)
		}

		render(w, tmpl, data)
	})

	if listenAddr == "" {
		listenAddr = "localhost:8090"
	}
	http.ListenAndServe(listenAddr, app)
}

func fetchVMDomains(store *registry.Store, clusterID string) []protocol.VMListNode {
	if store == nil || clusterID == "" {
		return nil
	}
	storeMasters := store.MastersForCluster(clusterID)
	log.Printf("orchestrator: vm page fetch cluster=%s masters=%d", clusterID, len(storeMasters))
	if len(storeMasters) == 0 {
		return nil
	}
	cmd := protocol.NodeCommand{Action: "libvirt.domains.list"}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return []protocol.VMListNode{{Error: "failed to build command"}}
	}
	results := make([]protocol.VMListNode, 0, len(storeMasters))
	for _, master := range storeMasters {
		if master.ID == "" {
			continue
		}
		reply, err := tcp.SendMasterCommandWait(master.ID, json.RawMessage(cmdBytes), 10*time.Second)
		if err != nil {
			results = append(results, protocol.VMListNode{NodeID: master.ID, Error: err.Error()})
			continue
		}
		if reply.Status != "ok" {
			results = append(results, protocol.VMListNode{NodeID: master.ID, Error: reply.Message})
			continue
		}
		aggregate := protocol.VMListAggregate{}
		if err := json.Unmarshal(reply.Output, &aggregate); err != nil {
			results = append(results, protocol.VMListNode{NodeID: master.ID, Error: "invalid domain list"})
			continue
		}
		results = append(results, aggregate.Nodes...)
	}
	return results
}
