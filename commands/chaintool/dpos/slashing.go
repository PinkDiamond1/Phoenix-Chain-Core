package dpos

import (
	"errors"

	"gopkg.in/urfave/cli.v1"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/p2p/discover"
)

var (
	SlashingCmd = cli.Command{
		Name:  "slashing",
		Usage: "use for slashing",
		Subcommands: []cli.Command{
			checkDuplicateSignCmd,
			zeroProduceNodeListCmd,
		},
	}
	checkDuplicateSignCmd = cli.Command{
		Name:   "checkDuplicateSign",
		Usage:  "3001,query whether the node has been reported for too many signatures,parameter:duplicateSignType,nodeid,blockNum",
		Before: netCheck,
		Action: checkDuplicateSign,
		Flags: []cli.Flag{rpcUrlFlag, addressHRPFlag,
			cli.Uint64Flag{
				Name:  "duplicateSignType",
				Usage: "duplicateSign type,1：prepareBlock，2：prepareVote，3：viewChange",
			},
			nodeIdFlag,
			blockNumFlag, jsonFlag,
		},
	}
	zeroProduceNodeListCmd = cli.Command{
		Name:   "zeroProduceNodeList",
		Usage:  "3002,query the list of nodes with zero block",
		Before: netCheck,
		Action: zeroProduceNodeList,
		Flags:  []cli.Flag{rpcUrlFlag, addressHRPFlag, jsonFlag},
	}
	blockNumFlag = cli.Uint64Flag{
		Name:  "blockNum",
		Usage: "blockNum",
	}
)

func checkDuplicateSign(c *cli.Context) error {
	duplicateSignType := c.Uint64("duplicateSignType")

	nodeIDstring := c.String(nodeIdFlag.Name)
	if nodeIDstring == "" {
		return errors.New("The reported node ID is not set")
	}
	nodeid, err := discover.HexID(nodeIDstring)
	if err != nil {
		return err
	}

	blockNum := c.Uint64(blockNumFlag.Name)

	return query(c, 3001, uint32(duplicateSignType), nodeid, blockNum)
}

func zeroProduceNodeList(c *cli.Context) error {
	return query(c, 3002)
}
