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
	}
}
