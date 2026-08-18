package buildinfo

var (
	Version         = "dev"
	Commit          = "unknown"
	BuildTime       = ""
	ProtocolVersion = "1"
)

func Capabilities() []string {
	return []string{
		"HOME",
		"DEDICATED",
		"TELEMETRY",
		"CONNECTIVITY_REPORTING",
		"VERSION_MANIFEST",
		"LIMIT_POLICY_ACCOUNT_HARD_LIMIT",
		"LIMIT_POLICY_INSTANCE_DIRECTIONAL",
		"LIMIT_POLICY_APPLY_CONFIRMATION",
	}
}
