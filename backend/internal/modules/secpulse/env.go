package secpulse

import "os"

// getEnv returns the value of the named environment variable.
func getEnv(key string) string {
	return os.Getenv(key)
}
