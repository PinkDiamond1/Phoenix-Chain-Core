package tests

import (
	"testing"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/vm"
)

func TestVM(t *testing.T) {
	t.Parallel()
	vmt := new(testMatcher)
	vmt.fails("^vmSystemOperationsTest.json/createNameRegistrator$", "fails without parallel execution")

	vmt.skipLoad(`^vmInputLimits(Light)?.json`) // log format broken

	vmt.skipShortMode("^vmPerformanceTest.json")
	vmt.skipShortMode("^vmInputLimits(Light)?.json")

	vmt.walk(t, vmTestDir, func(t *testing.T, name string, test *VMTest) {
		withTrace(t, test.json.Exec.GasLimit, func(vmconfig vm.Config) error {
			return vmt.checkFailure(t, name, test.Run(vmconfig))
		})
	})
}
