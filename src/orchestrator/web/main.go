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
	ActiveNode        registry.NodeInfo
	ActiveNodeItem    string

	NodesOpen     bool
	Clusters      []string
	ActiveCluster string
	ClusterOnline bool

	Nodes            []registry.NodeInfo
	VMDomains        []protocol.VMListNode
	DockerContainers []protocol.DockerContainer
	DockerError      string
}

type templates struct {
	home         *template.Template
	cluster      *template.Template
	vms          *template.Template
	reverseProxy *template.Template
	nodes        *template.Template
	node         *template.Template
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
	node, err := parsePage("page_node.html")
	if err != nil {
		return nil, err
	}

	return &templates{
		home:         home,
		cluster:      cluster,
		vms:          vms,
		reverseProxy: reverseProxy,
		nodes:        nodes,
		node:         node,
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
		if len(parts) < 1 || len(parts) > 4 {
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
		nodeItem := ""
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
			ActiveNodeItem:    nodeItem,
			ClusterOnline:     store.IsClusterOnline(id),
			Nodes:             nodes,
		}

		if len(parts) >= 3 {
			if parts[1] != "nodes" {
				http.NotFound(w, r)
				return
			}
			node, ok := findNode(nodes, parts[2])
			if !ok {
				http.NotFound(w, r)
				return
			}
			clusterItem = "nodes"
			data.ActiveClusterItem = clusterItem
			data.ActiveNode = node
			tmpl = tmpls.node
			title = "Node: " + nodeLabel(node)
			data.Title = title
			if len(parts) == 3 || (len(parts) == 4 && parts[3] == "docker") {
				nodeItem = "docker"
				data.ActiveNodeItem = nodeItem
				data.DockerContainers, data.DockerError = fetchNodeDocker(store, id, node.ID)
			} else {
				http.NotFound(w, r)
				return
			}
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

func findNode(nodes []registry.NodeInfo, raw string) (registry.NodeInfo, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return registry.NodeInfo{}, false
	}
	for _, node := range nodes {
		if node.ID == raw {
			return node, true
		}
	}
	return registry.NodeInfo{}, false
}

func nodeLabel(node registry.NodeInfo) string {
	if strings.TrimSpace(node.Name) != "" {
		return node.Name
	}
	return node.ID
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

func fetchNodeDocker(store *registry.Store, clusterID, nodeID string) ([]protocol.DockerContainer, string) {
	if store == nil || clusterID == "" || nodeID == "" {
		return nil, ""
	}
	storeMasters := store.MastersForCluster(clusterID)
	if len(storeMasters) == 0 {
		return nil, "master not connected"
	}
	cmd := protocol.NodeCommand{Action: "docker.containers.list"}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return nil, "failed to build docker command"
	}
	for _, master := range storeMasters {
		if master.ID == "" || !masterHasNode(master, nodeID) {
			continue
		}
		reply, err := tcp.SendCommandWait(master.ID, nodeID, json.RawMessage(cmdBytes), 10*time.Second)
		if err != nil {
			return nil, err.Error()
		}
		if reply.Status != "ok" {
			return nil, reply.Message
		}
		items := []protocol.DockerContainer{}
		if err := json.Unmarshal(reply.Output, &items); err != nil {
			return nil, "invalid docker container list"
		}
		return items, ""
	}
	return nil, "node not connected through any master"
}

func masterHasNode(master registry.MasterInfo, nodeID string) bool {
	for _, node := range master.Nodes {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}
