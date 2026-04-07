package version

var (
	Version = "0.2.1"
	Commit  = "dev"
	Date    = "unknown"
)

func String() string {
	return Version
}
