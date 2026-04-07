package version

var (
	Version = "0.2.2"
	Commit  = "dev"
	Date    = "unknown"
)

func String() string {
	return Version
}
