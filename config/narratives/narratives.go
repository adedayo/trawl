// Package narratives carries the curated, versioned plain-English content that
// explains findings (Change 025, Phase 3).
//
// The content is data rather than code because wording is a judgement that
// reviewers correct independently of the engine and of the libraries that raise
// findings. It is embedded as byte slices, like config/signals, so every
// entrypoint carries the same text inside itself; a narrative set that has to
// be found on disk is one that is absent in production, and an absent set
// would leave every finding unexplained.
package narratives

import (
	"embed"
	"sort"
)

//go:embed *.json
var files embed.FS

// JSON returns the raw content documents in a fixed order (by file name).
//
// Each call returns fresh copies. The embedded bytes are process-wide state,
// and handing out the backing arrays would let one caller's mishandling corrupt
// every later load.
func JSON() [][]byte {
	dirEntries, err := files.ReadDir(".")
	if err != nil {
		// The directory is embedded at build time; failing here means the
		// binary itself is malformed.
		panic("narratives: embedded directory unreadable: " + err.Error())
	}
	names := make([]string, 0, len(dirEntries))
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	out := make([][]byte, 0, len(names))
	for _, name := range names {
		data, err := files.ReadFile(name)
		if err != nil {
			panic("narratives: embedded file unreadable: " + err.Error())
		}
		cp := make([]byte, len(data))
		copy(cp, data)
		out = append(out, cp)
	}
	return out
}
