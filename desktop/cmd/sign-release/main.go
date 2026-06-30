package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	genkey := flag.Bool("genkey", false, "generate an ed25519 keypair and print base64 pub/priv")
	keyB64 := flag.String("key", "", "base64 ed25519 private key (or env UPDATE_SIGNING_PRIVKEY)")
	in := flag.String("in", "", "file to sign")
	out := flag.String("out", "", "signature output file (base64)")
	flag.Parse()

	if *genkey {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fail(err)
		}
		fmt.Println("PUBLIC_KEY_BASE64=" + base64.StdEncoding.EncodeToString(pub))
		fmt.Println("PRIVATE_KEY_BASE64=" + base64.StdEncoding.EncodeToString(priv))
		return
	}

	k := *keyB64
	if k == "" {
		k = os.Getenv("UPDATE_SIGNING_PRIVKEY")
	}
	if k == "" || *in == "" || *out == "" {
		fail(fmt.Errorf("usage: sign-release -key <b64|env UPDATE_SIGNING_PRIVKEY> -in <file> -out <file.sig>"))
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		fail(err)
	}
	sigB64, err := signData(k, data)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, []byte(sigB64), 0o644); err != nil {
		fail(err)
	}
}

// signData signs data with a std-base64 ed25519 private key and returns a
// std-base64 signature.
func signData(privB64 string, data []byte) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		return "", err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("bad private key size %d", len(raw))
	}
	sig := ed25519.Sign(ed25519.PrivateKey(raw), data)
	return base64.StdEncoding.EncodeToString(sig), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sign-release:", err)
	os.Exit(1)
}
