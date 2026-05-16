// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Command generate produces signed Vakt license keys.
//
// Usage:
//
//	go run ./backend/internal/license/generator/generate.go \
//	  --key /tmp/vakt_license.key \
//	  --org "Acme GmbH" \
//	  --tier pro \
//	  --features tisax,dora,audit_pdf \
//	  --expires 2027-12-31
package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type payload struct {
	Tier     string   `json:"tier"`
	Features []string `json:"features"`
	Org      string   `json:"org"`
	IssuedAt int64    `json:"iat"`
	Exp      *int64   `json:"exp,omitempty"`
}

func main() {
	keyFile := flag.String("key", "", "path to PEM-encoded ECDSA private key (required)")
	org := flag.String("org", "", "organisation name (required)")
	tier := flag.String("tier", "pro", "license tier: community | pro")
	featuresFlag := flag.String("features", "", "comma-separated feature list")
	expires := flag.String("expires", "", "expiry date YYYY-MM-DD (optional, omit for perpetual)")
	flag.Parse()

	if *keyFile == "" || *org == "" {
		flag.Usage()
		os.Exit(1)
	}

	privKey, err := loadPrivateKey(*keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "key load error: %v\n", err)
		os.Exit(1)
	}

	var features []string
	if *featuresFlag != "" {
		for _, f := range strings.Split(*featuresFlag, ",") {
			if f = strings.TrimSpace(f); f != "" {
				features = append(features, f)
			}
		}
	}

	p := payload{
		Tier:     *tier,
		Features: features,
		Org:      *org,
		IssuedAt: time.Now().UTC().Unix(),
	}

	if *expires != "" {
		t, err := time.Parse("2006-01-02", *expires)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --expires format (want YYYY-MM-DD): %v\n", err)
			os.Exit(1)
		}
		exp := t.UTC().Unix()
		p.Exp = &exp
	}

	payloadJSON, err := json.Marshal(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		os.Exit(1)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	hash := sha256.Sum256([]byte(payloadB64))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sign error: %v\n", err)
		os.Exit(1)
	}

	// Encode signature as fixed-width r||s (32 bytes each) for deterministic parsing.
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	fmt.Printf("%s.%s\n", payloadB64, sigB64)
}

func loadPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}
