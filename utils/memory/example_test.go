package memory_test

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/memory"
)

func ExampleTotalMemory() {
	fmt.Printf("Total system memory: %d\n", memory.TotalMemory())
}
func ExampleFreeMemory() {
	fmt.Printf("Free system memory: %d\n", memory.FreeMemory())
}
