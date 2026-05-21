package firewall

type Protocol string

const (
	udp Protocol = "UDP"
	tcp Protocol = "TCP"
)

type FirewallRule struct {
	Protocol Protocol
	Port     int
}
