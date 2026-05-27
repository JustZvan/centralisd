package web

import (
	"centralisd/src/core/protocol"
	"centralisd/src/orchestrator/registry"
	"centralisd/src/orchestrator/tcp"
	ctmpl "centralisd/templates"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
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
	DockerImages     []protocol.DockerImage
	DockerError      string
	DockerAction     string
	DockerActionError string
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
		log.Printf("web: render start title=%s cluster=%s node=%s item=%s masters=%d", data.Title, data.ActiveCluster, data.ActiveNode.ID, data.ActiveNodeItem, len(store.MastersForCluster(data.ActiveCluster)))
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			log.Printf("web: render error title=%s cluster=%s node=%s item=%s err=%v", data.Title, data.ActiveCluster, data.ActiveNode.ID, data.ActiveNodeItem, err)
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
		log.Printf("web: request method=%s path=%s", r.Method, r.URL.Path)
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
			log.Printf("web: unknown cluster id=%s", id)
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
				log.Printf("web: node not found cluster=%s node=%s", id, parts[2])
				http.NotFound(w, r)
				return
			}
			clusterItem = "nodes"
			data.ActiveClusterItem = clusterItem
			data.ActiveNode = node
			tmpl = tmpls.node
			title = "Node: " + nodeLabel(node)
			data.Title = title
			if len(parts) >= 3 && len(parts) <= 4 {
				sub := "docker"
				if len(parts) == 4 {
					sub = parts[3]
				}
				switch sub {
				case "docker", "docker-containers":
					nodeItem = "docker-containers"
					data.ActiveNodeItem = nodeItem
					if r.Method == http.MethodPost {
						if err := r.ParseForm(); err != nil {
							data.DockerActionError = "invalid form data"
							log.Printf("web: docker containers parse error cluster=%s node=%s err=%v", id, node.ID, err)
						} else {
							action := strings.TrimSpace(r.FormValue("action"))
							data.DockerAction = action
							log.Printf("web: docker containers action=%s cluster=%s node=%s", action, id, node.ID)
							data.DockerActionError = handleDockerContainersAction(store, id, node.ID, action, r)
							if data.DockerActionError != "" {
								log.Printf("web: docker containers action failed action=%s cluster=%s node=%s err=%s", action, id, node.ID, data.DockerActionError)
							}
						}
					}
					data.DockerContainers, data.DockerError = fetchNodeDocker(store, id, node.ID)
					if data.DockerError != "" {
						log.Printf("web: docker containers fetch error cluster=%s node=%s err=%s", id, node.ID, data.DockerError)
					}
				case "docker-images":
					nodeItem = "docker-images"
					data.ActiveNodeItem = nodeItem
					if r.Method == http.MethodPost {
						if err := r.ParseForm(); err != nil {
							data.DockerActionError = "invalid form data"
							log.Printf("web: docker images parse error cluster=%s node=%s err=%v", id, node.ID, err)
						} else {
							action := strings.TrimSpace(r.FormValue("action"))
							data.DockerAction = action
							log.Printf("web: docker images action=%s cluster=%s node=%s", action, id, node.ID)
							data.DockerActionError = handleDockerImagesAction(store, id, node.ID, action, r)
							if data.DockerActionError != "" {
								log.Printf("web: docker images action failed action=%s cluster=%s node=%s err=%s", action, id, node.ID, data.DockerActionError)
							}
						}
					}
					data.DockerImages, data.DockerError = fetchNodeDockerImages(store, id, node.ID)
					if data.DockerError != "" {
						log.Printf("web: docker images fetch error cluster=%s node=%s err=%s", id, node.ID, data.DockerError)
					}
				default:
					http.NotFound(w, r)
					return
				}
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
	cmd := protocol.NodeCommand{Action: "docker.containers.list"}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return nil, "failed to build docker command"
	}
	_, reply, err := tcp.SendClusterCommandWait(store, tcp.TargetRequest{
		ClusterID: clusterID,
		NodeID:    nodeID,
		Scope:     tcp.TargetMasterByNode,
	}, json.RawMessage(cmdBytes), 10*time.Second)
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

func fetchNodeDockerImages(store *registry.Store, clusterID, nodeID string) ([]protocol.DockerImage, string) {
	if store == nil || clusterID == "" || nodeID == "" {
		return nil, ""
	}
	cmd := protocol.NodeCommand{Action: "docker.images.list"}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return nil, "failed to build docker image command"
	}
	_, reply, err := tcp.SendClusterCommandWait(store, tcp.TargetRequest{
		ClusterID: clusterID,
		NodeID:    nodeID,
		Scope:     tcp.TargetMasterByNode,
	}, json.RawMessage(cmdBytes), 10*time.Second)
	if err != nil {
		return nil, err.Error()
	}
	if reply.Status != "ok" {
		return nil, reply.Message
	}
	items := []protocol.DockerImage{}
	if err := json.Unmarshal(reply.Output, &items); err != nil {
		return nil, "invalid docker image list"
	}
	return items, ""
}

func handleDockerContainersAction(store *registry.Store, clusterID, nodeID, action string, r *http.Request) string {
	if store == nil || clusterID == "" || nodeID == "" {
		return ""
	}
	containerID := strings.TrimSpace(r.FormValue("container_id"))
	if action == "create" {
		params := protocol.DockerContainerCreateParams{
			Image:         strings.TrimSpace(r.FormValue("image")),
			Name:          strings.TrimSpace(r.FormValue("name")),
			Command:       strings.TrimSpace(r.FormValue("command")),
			Entrypoint:    strings.TrimSpace(r.FormValue("entrypoint")),
			WorkingDir:    strings.TrimSpace(r.FormValue("working_dir")),
			Env:           splitLines(r.FormValue("env")),
			Ports:         splitCSV(r.FormValue("ports")),
			PublishAll:    formValueBool(r, "publish_all"),
			AutoRemove:    formValueBool(r, "auto_remove"),
			RestartPolicy: strings.TrimSpace(r.FormValue("restart_policy")),
			Start:         formValueBool(r, "start"),
		}
		if params.Image == "" {
			return "image is required"
		}
		payload, err := json.Marshal(params)
		if err != nil {
			return "failed to build create payload"
		}
		cmd := protocol.NodeCommand{Action: "docker.container.create", Params: payload}
		cmdBytes, err := json.Marshal(cmd)
		if err != nil {
			return "failed to build create command"
		}
		_, reply, err := tcp.SendClusterCommandWait(store, tcp.TargetRequest{
			ClusterID: clusterID,
			NodeID:    nodeID,
			Scope:     tcp.TargetMasterByNode,
		}, json.RawMessage(cmdBytes), 30*time.Second)
		if err != nil {
			return err.Error()
		}
		if reply.Status != "ok" {
			return reply.Message
		}
		return ""
	}
	if containerID == "" {
		return "container id is required"
	}
	cmdAction := ""
	params := map[string]any{"id": containerID}
	if action == "start" {
		cmdAction = "docker.container.start"
	} else if action == "stop" {
		cmdAction = "docker.container.stop"
		if timeout := strings.TrimSpace(r.FormValue("timeout")); timeout != "" {
			if value, err := parsePositiveInt(timeout); err == nil {
				params["timeout_seconds"] = value
			}
		}
	} else if action == "restart" {
		cmdAction = "docker.container.restart"
		if timeout := strings.TrimSpace(r.FormValue("timeout")); timeout != "" {
			if value, err := parsePositiveInt(timeout); err == nil {
				params["timeout_seconds"] = value
			}
		}
	} else if action == "remove" {
		cmdAction = "docker.container.remove"
		params["force"] = formValueBool(r, "force")
		params["remove_volumes"] = formValueBool(r, "remove_volumes")
	} else {
		return "unknown action"
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return "failed to build action payload"
	}
	cmd := protocol.NodeCommand{Action: cmdAction, Params: paramsBytes}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return "failed to build action command"
	}
	_, reply, err := tcp.SendClusterCommandWait(store, tcp.TargetRequest{
		ClusterID: clusterID,
		NodeID:    nodeID,
		Scope:     tcp.TargetMasterByNode,
	}, json.RawMessage(cmdBytes), 30*time.Second)
	if err != nil {
		return err.Error()
	}
	if reply.Status != "ok" {
		return reply.Message
	}
	return ""
}

func handleDockerImagesAction(store *registry.Store, clusterID, nodeID, action string, r *http.Request) string {
	if store == nil || clusterID == "" || nodeID == "" {
		return ""
	}
	image := strings.TrimSpace(r.FormValue("image"))
	if image == "" {
		return "image is required"
	}
	cmdAction := ""
	params := map[string]any{"image": image}
	if action == "pull" {
		cmdAction = "docker.image.pull"
	} else if action == "remove" {
		cmdAction = "docker.image.remove"
		params["force"] = formValueBool(r, "force")
		params["prune_children"] = formValueBool(r, "prune_children")
	} else {
		return "unknown action"
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return "failed to build action payload"
	}
	cmd := protocol.NodeCommand{Action: cmdAction, Params: paramsBytes}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return "failed to build action command"
	}
	_, reply, err := tcp.SendClusterCommandWait(store, tcp.TargetRequest{
		ClusterID: clusterID,
		NodeID:    nodeID,
		Scope:     tcp.TargetMasterByNode,
	}, json.RawMessage(cmdBytes), 30*time.Second)
	if err != nil {
		return err.Error()
	}
	if reply.Status != "ok" {
		return reply.Message
	}
	return ""
}

func formValueBool(r *http.Request, key string) bool {
	value := strings.ToLower(strings.TrimSpace(r.FormValue(key)))
	return value == "1" || value == "true" || value == "on" || value == "yes"
}

func splitLines(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func parsePositiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, errors.New("negative value")
	}
	return value, nil
}
