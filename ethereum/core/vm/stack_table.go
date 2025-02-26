package vm

import (
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/configs"
)

func minSwapStack(n int) int {
	return minStack(n, n)
}
func maxSwapStack(n int) int {
	return maxStack(n, n)
}

func minDupStack(n int) int {
	return minStack(n, n+1)
}
func maxDupStack(n int) int {
	return maxStack(n, n+1)
}

func maxStack(pop, push int) int {
	return int(configs.StackLimit) + pop - push
}
func minStack(pops, push int) int {
	return pops
}
