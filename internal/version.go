package internal

import "runtime/debug"

// Version is the application version shown in the footer.
//
// Resolved as follows:
//
//	Install method                                    | Resolved version
//	--------------------------------------------------|---------------------------------
//	go install github.com/mahlburgc/teaterm@v0.1.0    | v0.1.0 (the resolved tag)
//	go install github.com/mahlburgc/teaterm@latest    | highest semver tag in the repo
//	go install github.com/mahlburgc/teaterm@main      | pseudo-version (v0.0.0-<date>-<sha>)
//	go run . / go build from a clone                  | 0.0.1-dev
//	go build -ldflags "-X .../internal.Version=vX"    | vX
//
// Must remain a `var` initialized to a string literal so -X can override it.
var Version = "0.0.1-dev"

func init() {
	if Version != "0.0.1-dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			Version = v
		}
	}
}
