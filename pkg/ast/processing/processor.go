package processing

import (
	"github.com/google/go-jsonnet"
	"github.com/grafana/jsonnet-language-server/pkg/cache"
)

type Processor struct {
	cache *cache.Cache
	vm    *jsonnet.VM
	paths []string
}

func NewProcessor(cache *cache.Cache, vm *jsonnet.VM, paths []string) *Processor {
	return &Processor{
		cache: cache,
		vm:    vm,
		paths: paths,
	}
}
