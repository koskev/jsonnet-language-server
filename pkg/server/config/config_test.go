package config

import (
	"encoding/json"
	"testing"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	testCases := []struct {
		name       string
		config     string
		workspaces []protocol.WorkspaceFolder

		expectError    bool
		expectedConfig Configuration
		// Since go is such an awesome language which does not know defaults, we use this awful workaround
		changedFormatterOptions bool
	}{
		{
			name:   "empty",
			config: "{}",

			expectedConfig: Configuration{},
		},
		{
			name: "log config",
			config: `
			{
				"log_level": "error"
			}
			`,
			expectedConfig: Configuration{
				LogLevel: 2,
			},
		},
		{
			name: "relative jpath",
			config: `
			{
				"paths": {
					"relative_jpaths": ["lib", "vendor"]
				},
				"jpath": ["/test"]
			}
			`,
			workspaces: []protocol.WorkspaceFolder{{Name: "/test"}, {Name: "/test2"}},
			expectedConfig: Configuration{
				JPaths: []string{
					"/test",
					"/test/lib",
					"/test/vendor",
					"/test2/lib",
					"/test2/vendor",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conf := Configuration{workspaces: tc.workspaces}
			tc.expectedConfig.workspaces = tc.workspaces
			err := json.Unmarshal([]byte(tc.config), &conf)
			assert.NoError(t, err)
			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			// GO is an awesome language that does not have defaults....
			if !tc.changedFormatterOptions {
				tc.expectedConfig.FormattingOptions, err = parseFormattingOpts(map[string]any{})
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedConfig, conf)
		})
	}
}
