package dpos

import (
	"gopkg.in/urfave/cli.v1"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/p2p/discover"
)

var (
	RewardCmd = cli.Command{
		Name:  "reward",
		Usage: "use for reward",
		Subcommands: []cli.Command{
			getDelegateRewardCmd,
		},
	}
	getDelegateRewardCmd = cli.Command{
		Name:   "getDelegateReward",
		Usage:  "5100,query account not withdrawn commission rewards at each node,parameter:nodeList(can empty)",
		Before: netCheck,
		Action: getDelegateReward,
		Flags:  []cli.Flag{rpcUrlFlag, addressHRPFlag, nodeList, jsonFlag},
	}
	nodeList = cli.StringSliceFlag{
		Name:  "nodeList",
		Usage: "node list,may empty",
	}
)

func getDelegateReward(c *cli.Context) error {
	nodeIDlist := c.StringSlice(nodeList.Name)
	idlist := make([]discover.NodeID, 0)
	for _, node := range nodeIDlist {
		nodeid, err := discover.HexID(node)
		if err != nil {
			return err
		}
		idlist = append(idlist, nodeid)
	}
	return query(c, 5100, idlist)
}
