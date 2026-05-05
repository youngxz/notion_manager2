package netutil

import (
	"math/rand"
	"time"

	utls "github.com/refraction-networking/utls"
)

var chromeProfiles = []utls.ClientHelloID{
	utls.HelloChrome_120,
	utls.HelloChrome_131,
	utls.HelloChrome_133,
}

var chromeVersions = []string{
	"120.0.0.0",
	"131.0.0.0",
	"133.0.0.0",
}

var majorVersions = []string{
	"120", "131", "133",
}

var (
	currentProfile  utls.ClientHelloID
	currentFullVer  string
	currentMajorVer string
)

func init() {
	rand.Seed(time.Now().UnixNano())
	RotateChromeProfile()
}

// RotateChromeProfile randomizes the global Chrome profile settings.
func RotateChromeProfile() {
	idx := rand.Intn(len(chromeProfiles))
	currentProfile = chromeProfiles[idx]
	currentFullVer = chromeVersions[idx]
	currentMajorVer = majorVersions[idx]
}

// GetRandomChromeProfile returns a newly randomized uTLS Chrome profile each time.
func GetRandomChromeProfile() (utls.ClientHelloID, string, string) {
	idx := rand.Intn(len(chromeProfiles))
	return chromeProfiles[idx], chromeVersions[idx], majorVersions[idx]
}

// GetCurrentChromeProfile returns the current globally selected Chrome profile for the session
func GetCurrentChromeProfile() (utls.ClientHelloID, string, string) {
	return currentProfile, currentFullVer, currentMajorVer
}

// GenerateUserAgent returns a generated User-Agent string for the given Chrome version.
func GenerateUserAgent(fullVersion string) string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + fullVersion + " Safari/537.36"
}

// GenerateSecChUa returns a generated sec-ch-ua header for the given major version.
func GenerateSecChUa(majorVersion string) string {
	return `"Chromium";v="` + majorVersion + `", "Not(A:Brand";v="24", "Google Chrome";v="` + majorVersion + `"`
}
