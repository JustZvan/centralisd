package types

type HeartbeatRespUsage struct {
	CPU int `json:"cpu"`
	RAM int `json:"ram"`
}

type HeartbeatRespTotalHardware struct {
	CPU int `json:"cpu"`
	RAM int `json:"ram"`
}

type HeartbeatRespPacket struct {
	Usage    HeartbeatRespUsage         `json:"usage"`
	Hardware HeartbeatRespTotalHardware `json:"hardware"`
}
