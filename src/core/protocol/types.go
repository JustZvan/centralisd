package protocol

import "encoding/json"

type PacketType string

const (
    PacketError             PacketType = "error"
    PacketAuthHello         PacketType = "auth.hello"
    PacketAuthChallenge     PacketType = "auth.challenge"
    PacketAuthProof         PacketType = "auth.proof"
    PacketAuthOK            PacketType = "auth.ok"
    PacketHeartbeat         PacketType = "heartbeat"
    PacketHeartbeatReply    PacketType = "heartbeat.reply"
    PacketNodeCommand       PacketType = "node.command"
    PacketNodeCommandReply  PacketType = "node.command.reply"
    PacketOrchCommand       PacketType = "orchestrator.command"
    PacketOrchCommandReply  PacketType = "orchestrator.command.reply"
    PacketMasterRegister    PacketType = "master.register"
    PacketMasterHeartbeat   PacketType = "master.heartbeat"
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
	ID     uint32 `json:"id"`
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type VMListNode struct {
	NodeID  string     `json:"node_id"`
	Domains []VMDomain `json:"domains,omitempty"`
	Error   string     `json:"error,omitempty"`
}

type VMListAggregate struct {
	Nodes []VMListNode `json:"nodes"`
}
