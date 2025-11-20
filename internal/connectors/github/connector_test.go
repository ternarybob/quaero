package github

import (
	"encoding/json"
	"testing"

	"github.com/ternarybob/quaero/internal/models"
)

func TestNewConnector(t *testing.T) {
	tests := []struct {
		name      string
		configMap map[string]interface{}
		typeStr   string
		wantErr   bool
	}{
		{
			name:    "Valid Config",
			typeStr: string(models.ConnectorTypeGitHub),
			configMap: map[string]interface{}{
				"token": "ghp_validtoken",
			},
			wantErr: false,
		},
		{
			name:    "Invalid Type",
			typeStr: "invalid",
			configMap: map[string]interface{}{
				"token": "ghp_validtoken",
			},
			wantErr: true,
		},
		{
			name:      "Missing Token",
			typeStr:   string(models.ConnectorTypeGitHub),
			configMap: map[string]interface{}{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configJSON, _ := json.Marshal(tt.configMap)
			connector := &models.Connector{
				Type:   models.ConnectorType(tt.typeStr),
				Config: configJSON,
			}

			_, err := NewConnector(connector)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
