package conf_test

import (
	"testing"

	. "github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/proxy/http"
)

func TestHTTPServerConfig(t *testing.T) {
	creator := func() Buildable {
		return new(HTTPServerConfig)
	}

	runMultiTestCase(t, []TestCase{
		{
			Input: `{
				"accounts": [
					{
						"user": "my-username",
						"pass": "my-password"
					}
				],
				"allowTransparent": true,
				"userLevel": 1
			}`,
			Parser: loadJSON(creator),
			Output: &http.ServerConfig{
				UserAccounts: []*http.UserAccount{
					{
						Username: "my-username",
						Password: "my-password",
					},
				},
				AllowTransparent: true,
				UserLevel:        1,
			},
		},
	})
}
