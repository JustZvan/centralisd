package firewall

import (
	"centralisd/src/core/config"
	"net"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func SetupFirewall(config config.Config) error {
	nft, err := nftables.New()
	if err != nil {
		return err
	}

	nft.FlushRuleset()

	table := &nftables.Table{
		Name:   "filter",
		Family: nftables.TableFamilyINet,
	}
	nft.AddTable(table)

	policy := nftables.ChainPolicyDrop

	input := &nftables.Chain{
		Name:     "input",
		Table:    table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookInput,
		Priority: nftables.ChainPriorityFilter,
		Policy:   &policy,
	}
	nft.AddChain(input)

	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: input,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyIIFNAME,
				Register: 1,
			},
			&expr.Cmp{
				Register: 1,
				Op:       expr.CmpOpEq,
				Data:     []byte("lo\x00"),
			},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	})

	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: input,
		Exprs: []expr.Any{
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       12,
				Len:          4,
			},
			&expr.Cmp{
				Register: 1,
				Op:       expr.CmpOpEq,
				Data:     net.ParseIP(strings.Split(config.Slave.Master, ":")[1]).To4(),
			},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	})

	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: input,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: 1,
			},
			&expr.Cmp{
				Register: 1,
				Op:       expr.CmpOpEq,
				Data:     []byte{6}, // TCP
			},
			&expr.Payload{
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
				DestRegister: 1,
			},
			&expr.Cmp{
				Register: 1,
				Op:       expr.CmpOpEq,
				Data:     []byte{0, 22},
			},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	})

	return nft.Flush()
}
