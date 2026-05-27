package protocol

import "encoding/json"

type PacketType string

const (
	PacketError            PacketType = "error"
	PacketAuthHello        PacketType = "auth.hello"
	PacketAuthChallenge    PacketType = "auth.challenge"
	PacketAuthProof        PacketType = "auth.proof"
	PacketAuthOK           PacketType = "auth.ok"
	PacketHeartbeat        PacketType = "heartbeat"
	PacketHeartbeatReply   PacketType = "heartbeat.reply"
	PacketNodeCommand      PacketType = "node.command"
	PacketNodeCommandReply PacketType = "node.command.reply"
	PacketOrchCommand      PacketType = "orchestrator.command"
	PacketOrchCommandReply PacketType = "orchestrator.command.reply"
	PacketMasterRegister   PacketType = "master.register"
	PacketMasterHeartbeat  PacketType = "master.heartbeat"
)

type AuthHello struct {
	ID        string `json:"id"`
	PubKey    string `json:"pubkey"`
	Role      string `json:"role"`
	Name      string `json:"name,omitempty"`
	Cluster   string `json:"cluster,omitempty"`
	Advertise string `json:"advertise,omitempty"`
}

type AuthChallenge struct {
	Challenge string `json:"challenge"`
}

type AuthProof struct {
	Signature string `json:"signature"`
}

type MasterInfo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Cluster   string     `json:"cluster"`
	Advertise string     `json:"advertise"`
	PubKey    string     `json:"pubKey"`
	Nodes     []NodeInfo `json:"nodes"`
}

type NodeInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type Heartbeat struct {
	Usage    HeartbeatUsage    `json:"usage"`
	Hardware HeartbeatHardware `json:"hardware"`
}

type HeartbeatUsage struct {
	CPUPercent float64 `json:"cpu_percent"`
	RAMPercent float64 `json:"ram_percent"`
}

type HeartbeatHardware struct {
	CPUCores int    `json:"cpu_cores"`
	RAMBytes uint64 `json:"ram_bytes"`
}

type OrchestratorCommand struct {
	NodeID  string          `json:"node_id"`
	Command json.RawMessage `json:"command"`
}

type CommandReply struct {
	NodeID  string          `json:"node_id,omitempty"`
	Status  string          `json:"status"`
	Message string          `json:"message,omitempty"`
	Output  json.RawMessage `json:"output,omitempty"`
}

type NodeCommand struct {
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
}

type VMDomain struct {
	ID         uint32 `json:"id"`
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	Persistent bool   `json:"persistent,omitempty"`
	Autostart  bool   `json:"autostart,omitempty"`
	State      string `json:"state,omitempty"`
	MaxMemMB   uint64 `json:"max_mem_mb,omitempty"`
	MemoryMB   uint64 `json:"memory_mb,omitempty"`
	VCPUs      uint   `json:"vcpus,omitempty"`
	ISOPath    string `json:"iso_path,omitempty"`
}

type VMListNode struct {
	NodeID  string     `json:"node_id"`
	Domains []VMDomain `json:"domains,omitempty"`
	Error   string     `json:"error,omitempty"`
}

type VMListAggregate struct {
	Nodes []VMListNode `json:"nodes"`
}

type LibvirtStorageVolume struct {
	Name       string `json:"name"`
	Key        string `json:"key,omitempty"`
	Path       string `json:"path,omitempty"`
	Format     string `json:"format,omitempty"`
	Capacity   uint64 `json:"capacity,omitempty"`
	Allocation uint64 `json:"allocation,omitempty"`
}

type LibvirtStoragePool struct {
	Name       string                 `json:"name"`
	UUID       string                 `json:"uuid,omitempty"`
	Active     bool                   `json:"active,omitempty"`
	Persistent bool                   `json:"persistent,omitempty"`
	Autostart  bool                   `json:"autostart,omitempty"`
	Capacity   uint64                 `json:"capacity,omitempty"`
	Allocation uint64                 `json:"allocation,omitempty"`
	Available  uint64                 `json:"available,omitempty"`
	Volumes    []LibvirtStorageVolume `json:"volumes,omitempty"`
}

type LibvirtNetwork struct {
	Name       string `json:"name"`
	UUID       string `json:"uuid,omitempty"`
	Active     bool   `json:"active,omitempty"`
	Persistent bool   `json:"persistent,omitempty"`
	Autostart  bool   `json:"autostart,omitempty"`
}

type LibvirtVMActionParams struct {
	Name string `json:"name"`
}

type LibvirtVMCreateParams struct {
	Name       string `json:"name"`
	MemoryMB   uint64 `json:"memory_mb"`
	VCPUs      uint   `json:"vcpus"`
	DiskPool   string `json:"disk_pool"`
	DiskName   string `json:"disk_name,omitempty"`
	DiskSizeGB uint64 `json:"disk_size_gb"`
	Network    string `json:"network,omitempty"`
	ISOPath    string `json:"iso_path,omitempty"`
	Autostart  bool   `json:"autostart,omitempty"`
	Start      bool   `json:"start,omitempty"`
}

type LibvirtVMCreateResult struct {
	Name       string `json:"name"`
	UUID       string `json:"uuid,omitempty"`
	DiskVolume string `json:"disk_volume,omitempty"`
	DiskPath   string `json:"disk_path,omitempty"`
}

type LibvirtVMSetAutostartParams struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type LibvirtVMAttachISOParams struct {
	Name    string `json:"name"`
	ISOPath string `json:"iso_path"`
}

type LibvirtVMDeleteParams struct {
	Name          string `json:"name"`
	ForceStop     bool   `json:"force_stop,omitempty"`
	RemoveVolumes bool   `json:"remove_volumes,omitempty"`
}

type DockerContainer struct {
	ID      string   `json:"id"`
	Image   string   `json:"image"`
	Names   []string `json:"names,omitempty"`
	State   string   `json:"state,omitempty"`
	Status  string   `json:"status,omitempty"`
	Created int64    `json:"created,omitempty"`
}

type DockerImage struct {
	ID       string            `json:"id"`
	RepoTags []string          `json:"repo_tags,omitempty"`
	Size     int64             `json:"size,omitempty"`
	Created  int64             `json:"created,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type DockerImagePullParams struct {
	Image string `json:"image"`
}

type DockerImageRemoveParams struct {
	Image         string `json:"image"`
	Force         bool   `json:"force,omitempty"`
	PruneChildren bool   `json:"prune_children,omitempty"`
}

type DockerImageRemoveResult struct {
	Deleted  []string `json:"deleted,omitempty"`
	Untagged []string `json:"untagged,omitempty"`
}

type DockerContainerCreateParams struct {
	Image         string   `json:"image"`
	Name          string   `json:"name,omitempty"`
	Command       string   `json:"command,omitempty"`
	Entrypoint    string   `json:"entrypoint,omitempty"`
	WorkingDir    string   `json:"working_dir,omitempty"`
	Env           []string `json:"env,omitempty"`
	Ports         []string `json:"ports,omitempty"`
	PublishAll    bool     `json:"publish_all,omitempty"`
	AutoRemove    bool     `json:"auto_remove,omitempty"`
	RestartPolicy string   `json:"restart_policy,omitempty"`
	Start         bool     `json:"start,omitempty"`
}

type DockerContainerCreateResult struct {
	ID       string   `json:"id"`
	Warnings []string `json:"warnings,omitempty"`
}

type DockerContainerStartParams struct {
	ID string `json:"id"`
}

type DockerContainerStopParams struct {
	ID             string `json:"id"`
	TimeoutSeconds *int   `json:"timeout_seconds,omitempty"`
}

type DockerContainerRestartParams struct {
	ID             string `json:"id"`
	TimeoutSeconds *int   `json:"timeout_seconds,omitempty"`
}

type DockerContainerRemoveParams struct {
	ID            string `json:"id"`
	Force         bool   `json:"force,omitempty"`
	RemoveVolumes bool   `json:"remove_volumes,omitempty"`
}
