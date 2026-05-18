package firewall

type Protocol string

const (
	TCP Protocol = "tcp"
	UDP Protocol = "udp"
)

type FirewallRule struct {
	port     int
	protocol Protocol
}

func GenerateRuleSet(rules []FirewallRule) string {
	return ""
}
func ApplyRuleSet(ruleset string) {

}
