package hardware

import (
	"log"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type HardwareInfo struct {
	RAM *mem.VirtualMemoryStat
	CPU float64
	CPUCores int
}

func GetHardwareInfo() HardwareInfo {
	ram, err := mem.VirtualMemory()

	if err != nil {
		ram = &mem.VirtualMemoryStat{
			Free:        0,
			Total:       0,
			UsedPercent: 0,
		}
	}

	cpu_usage, err := cpu.Percent(3*time.Second, false)

	if err != nil {
		log.Printf("hardware: get hardware info: %v", err)
	}

	cores, _ := cpu.Counts(true)

	return HardwareInfo{
		CPU:      cpu_usage[0],
		RAM:      ram,
		CPUCores: cores,
	}
}
