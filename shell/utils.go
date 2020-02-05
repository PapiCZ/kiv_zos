package shell

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"strconv"
)

func ClusterPtrsToStrings(ptrs []vfs.ClusterPtr) []string {
	strs := make([]string, 0)
	for _, ptr := range ptrs {
		strs = append(strs, strconv.Itoa(int(ptr)))
	}

	return strs
}
