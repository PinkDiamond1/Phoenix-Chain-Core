package rpc_test

import (
	"context"
	"fmt"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common/hexutil"
	"time"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rpc"
)

// In this example, our client wishes to track the latest 'block number'
// known to the server. The server supports two methods:
//
// phoenixchain_getBlockByNumber("latest", {})
//    returns the latest block object.
//
// phoenixchain_subscribe("newBlocks")
//    creates a subscription which fires block objects when new blocks arrive.

type Block struct {
	Number *hexutil.Big
}

func ExampleClientSubscription() {
	// Connect the client.
	client, _ := rpc.Dial("ws://127.0.0.1:8485")
	subch := make(chan Block)

	// Ensure that subch receives the latest block.
	go func() {
		for i := 0; ; i++ {
			if i > 0 {
				time.Sleep(2 * time.Second)
			}
			subscribeBlocks(client, subch)
		}
	}()

	// Print events from the subscription as they arrive.
	for block := range subch {
		fmt.Println("latest block:", block.Number)
	}
}

// subscribeBlocks runs in its own goroutine and maintains
// a subscription for new blocks.
func subscribeBlocks(client *rpc.Client, subch chan Block) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Subscribe to new blocks.
	sub, err := client.EthSubscribe(ctx, subch, "newHeads")
	if err != nil {
		fmt.Println("subscribe error:", err)
		return
	}

	// The connection is established now.
	// Update the channel with the current block.
	var lastBlock Block
	if err := client.CallContext(ctx, &lastBlock, "phoenixchain_getBlockByNumber", "latest", false); err != nil {
		fmt.Println("can't get latest block:", err)
		return
	}
	subch <- lastBlock

	// The subscription will deliver events to the channel. Wait for the
	// subscription to end for any reason, then loop around to re-establish
	// the connection.
	fmt.Println("connection lost: ", <-sub.Err())
}
