package version

var (
	Version = "0.2.5"
	Commit  = "dev"
	Date    = "unknown"
)

func String() string {
	return Version
}
