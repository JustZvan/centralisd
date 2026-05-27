package handlers

import (
	"bufio"
	"centralisd/src/core/protocol"
	"centralisd/src/slave/hardware"
	"log"
)

func handleHeartbeat(writer *bufio.Writer, packet protocol.Packet) error {
	log.Printf("slave: heartbeat")
	hw := hardware.GetHardwareInfo()
	heartbeat := protocol.Heartbeat{
		Usage: protocol.HeartbeatUsage{
			CPUPercent: hw.CPU,
			RAMPercent: hw.RAM.UsedPercent,
		},
		Hardware: protocol.HeartbeatHardware{
			CPUCores: int(hw.CPUCores),
			RAMBytes: hw.RAM.Total,
		},
	}

	reply, err := protocol.NewReply(string(protocol.PacketHeartbeatReply), packet.ID, heartbeat)
	if err != nil {
		return err
	}

	return protocol.WritePacket(writer, reply)
}
