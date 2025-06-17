package server

import (
	"context"

	"github.com/google/go-jsonnet"
	"github.com/grafana/jsonnet-language-server/pkg/server/config"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

// TODO: are the params all settings or just the changed settings?
//
//nolint:gocyclo
func (s *Server) DidChangeConfiguration(_ context.Context, params *protocol.DidChangeConfigurationParams) error {
	settings, err := config.NewConfiguration(params.Settings, s.initParams.WorkspaceFolders)
	if err != nil {
		return err
	}

	log.SetLevel(settings.LogLevel)
	s.configuration = *settings

	log.Infof("configuration updated: %+v", s.configuration)

	return nil
}

func resetExtVars(vm *jsonnet.VM, vars map[string]string, code map[string]string) {
	vm.ExtReset()
	for vk, vv := range vars {
		vm.ExtVar(vk, vv)
	}
	for vk, vv := range code {
		vm.ExtCode(vk, vv)
	}
}
