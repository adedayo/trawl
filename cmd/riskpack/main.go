// Command riskpack manages detached signatures for versioned model packs.
// Private keys are supplied by the release operator and are never inferred
// from repository state.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"flag"
	"fmt"
	"os"

	"github.com/adedayo/trawl/pkg/riskpack"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "keygen":
		keygen(os.Args[2:])
	case "sign":
		sign(os.Args[2:])
	case "verify":
		verify(os.Args[2:])
	default:
		usage()
	}
}

func keygen(args []string) {
	flags := flag.NewFlagSet("keygen", flag.ExitOnError)
	privatePath := flags.String("private-key", "", "path for the private key")
	publicPath := flags.String("public-key", "", "path for the public key")
	flags.Parse(args)
	if *privatePath == "" || *publicPath == "" {
		fail("keygen requires --private-key and --public-key")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*privatePath, private, 0o600); err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*publicPath, public, 0o644); err != nil {
		fail(err.Error())
	}
}

func sign(args []string) {
	flags := flag.NewFlagSet("sign", flag.ExitOnError)
	packPath := flags.String("pack", "", "pack JSON path")
	privatePath := flags.String("private-key", "", "Ed25519 private key path")
	signaturePath := flags.String("signature", "", "detached signature output path")
	flags.Parse(args)
	if *packPath == "" || *privatePath == "" || *signaturePath == "" {
		fail("sign requires --pack, --private-key, and --signature")
	}
	contents, err := os.ReadFile(*packPath)
	if err != nil {
		fail(err.Error())
	}
	if _, err := riskpack.ValidateBytes(contents); err != nil {
		fail(err.Error())
	}
	key, err := os.ReadFile(*privatePath)
	if err != nil {
		fail(err.Error())
	}
	if len(key) != ed25519.PrivateKeySize {
		fail("private key is not an Ed25519 private key")
	}
	if err := os.WriteFile(*signaturePath, ed25519.Sign(ed25519.PrivateKey(key), contents), 0o644); err != nil {
		fail(err.Error())
	}
}

func verify(args []string) {
	flags := flag.NewFlagSet("verify", flag.ExitOnError)
	packPath := flags.String("pack", "", "pack JSON path")
	publicPath := flags.String("public-key", "", "Ed25519 public key path")
	signaturePath := flags.String("signature", "", "detached signature path")
	flags.Parse(args)
	if *packPath == "" || *publicPath == "" || *signaturePath == "" {
		fail("verify requires --pack, --public-key, and --signature")
	}
	key, err := os.ReadFile(*publicPath)
	if err != nil {
		fail(err.Error())
	}
	if len(key) != ed25519.PublicKeySize {
		fail("public key is not an Ed25519 public key")
	}
	if _, err := riskpack.Load(*packPath, *signaturePath, ed25519.PublicKey(key)); err != nil {
		fail(err.Error())
	}
	fmt.Println("risk pack signature verified")
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: riskpack keygen|sign|verify")
	os.Exit(2)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "riskpack:", message)
	os.Exit(2)
}
