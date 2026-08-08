package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/adedayo/trawl/pkg/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the whole CLI, expressed as a function over its arguments and output
// streams so that the command surface can be tested without spawning a process
// or intercepting os.Exit. main() is then thin enough to be correct by
// inspection, which is the only kind of correctness available to code that
// cannot be tested.
func run(args []string, stdout, stderr io.Writer) int {
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "server":
		// Emitted before the listener binds, so that a container whose server
		// fails to start still leaves its build identity in the logs.
		// Diagnosing a crash-looping worker is materially harder when the only
		// thing the log establishes is that something crashed.
		log.Printf("Trawl %s — starting Cloud Continuous EASM Server...", version.Get())
		runServer()
		return 0

	case "version", "--version", "-v":
		fmt.Fprint(stdout, version.Get().Long("trawl"))
		return 0

	case "help", "--help", "-h":
		usage(stdout)
		return 0

	default:
		if cmd != "" {
			fmt.Fprintf(stderr, "trawl: unknown command %q\n\n", cmd)
		}
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `trawl — continuous external attack surface monitoring

Usage:
  trawl server     Run the headless ingest server and job broker
  trawl version    Print build version, commit and platform
  trawl help       Show this message
`)
}
