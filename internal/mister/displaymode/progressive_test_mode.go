package displaymode

import "os"

// ProgressiveTestRequested enables the isolated diagnostic supervisor once.
// Only its supervised child receives the native page-flipping environment flag.
func ProgressiveTestRequested() bool {
	return os.Getenv("MISTERVISION_PROGRESSIVE_TEST") == "1" && os.Getenv(scaledEnv) != "1"
}
