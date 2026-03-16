package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/atproto/atcrypto"

	"github.com/urfave/cli/v3"
)

// PrivateJWK extends JWK with the "d" parameter for private key material.
// This is used for serializing private keys in JWK format.
type PrivateJWK struct {
	KeyType string `json:"kty"`
	Curve   string `json:"crv"`
	X       string `json:"x"`             // base64url, no padding
	Y       string `json:"y"`             // base64url, no padding
	D       string `json:"d"`             // base64url, no padding (private key)
	Use     string `json:"use,omitempty"`
}

var cmdKey = &cli.Command{
	Name:  "key",
	Usage: "commands for managing cryptographic keys",
	Commands: []*cli.Command{
		&cli.Command{
			Name:  "generate",
			Usage: "create a new secret key",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "type",
					Aliases: []string{"t"},
					Usage:   "indicate curve type (P-256 is default)",
				},
				&cli.BoolFlag{
					Name:  "terse",
					Usage: "print just the secret key, in multikey format",
				},
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "output format: plain (default) or jwk",
				},
			},
			Action: runKeyGenerate,
		},
		&cli.Command{
			Name:      "inspect",
			Usage:     "parses and outputs metadata about a public or secret key",
			ArgsUsage: `<key>`,
			Action:    runKeyInspect,
		},
	},
}

func runKeyGenerate(ctx context.Context, cmd *cli.Command) error {
	var priv atcrypto.PrivateKeyExportable
	switch cmd.String("type") {
	case "", "P-256", "p256", "ES256", "secp256r1":
		sec, err := atcrypto.GeneratePrivateKeyP256()
		if err != nil {
			return err
		}
		priv = sec
	case "K-256", "k256", "ES256K", "secp256k1":
		sec, err := atcrypto.GeneratePrivateKeyK256()
		if err != nil {
			return err
		}
		priv = sec
	default:
		return fmt.Errorf("unknown key type: %s", cmd.String("type"))
	}

	outputFormat := cmd.String("output")
	switch outputFormat {
	case "", "plain":
		if cmd.Bool("terse") {
			fmt.Println(priv.Multibase())
			return nil
		}
		pub, err := priv.PublicKey()
		if err != nil {
			return err
		}
		fmt.Printf("Key Type: %s\n", descKeyType(priv))
		fmt.Printf("Secret Key (Multibase Syntax): save this securely (eg, add to password manager)\n\t%s\n", priv.Multibase())
		fmt.Printf("Public Key (DID Key Syntax): share or publish this (eg, in DID document)\n\t%s\n", pub.DIDKey())
		return nil
	case "jwk":
		jwk, err := privateKeyToJWK(priv)
		if err != nil {
			return err
		}
		out, err := json.MarshalIndent(jwk, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	default:
		return fmt.Errorf("unknown output format: %s (use 'plain' or 'jwk')", outputFormat)
	}
}

// privateKeyToJWK converts a private key to JWK format.
func privateKeyToJWK(priv atcrypto.PrivateKeyExportable) (*PrivateJWK, error) {
	pub, err := priv.PublicKey()
	if err != nil {
		return nil, err
	}
	pubJWK, err := pub.JWK()
	if err != nil {
		return nil, err
	}

	// The private key bytes are the scalar "d" value
	dBytes := priv.Bytes()

	return &PrivateJWK{
		KeyType: pubJWK.KeyType,
		Curve:   pubJWK.Curve,
		X:       pubJWK.X,
		Y:       pubJWK.Y,
		D:       base64.RawURLEncoding.EncodeToString(dBytes),
	}, nil
}

func descKeyType(val interface{}) string {
	switch val.(type) {
	case *atcrypto.PublicKeyP256, atcrypto.PublicKeyP256:
		return "P-256 / secp256r1 / ES256 public key"
	case *atcrypto.PrivateKeyP256, atcrypto.PrivateKeyP256:
		return "P-256 / secp256r1 / ES256 private key"
	case *atcrypto.PublicKeyK256, atcrypto.PublicKeyK256:
		return "K-256 / secp256k1 / ES256K public key"
	case *atcrypto.PrivateKeyK256, atcrypto.PrivateKeyK256:
		return "K-256 / secp256k1 / ES256K private key"
	default:
		return "unknown"
	}
}

func runKeyInspect(ctx context.Context, cmd *cli.Command) error {
	s := cmd.Args().First()
	if s == "" {
		return fmt.Errorf("need to provide key as an argument")
	}

	// Try parsing as JWK (JSON format)
	if strings.HasPrefix(strings.TrimSpace(s), "{") {
		return inspectJWK(s)
	}

	sec, err := atcrypto.ParsePrivateMultibase(s)
	if nil == err {
		fmt.Printf("Type: %s\n", descKeyType(sec))
		fmt.Printf("Encoding: multibase\n")
		pub, err := sec.PublicKey()
		if err != nil {
			return err
		}
		fmt.Printf("Public (DID Key): %s\n", pub.DIDKey())
		return nil
	}

	pub, err := atcrypto.ParsePublicMultibase(s)
	if nil == err {
		fmt.Printf("Type: %s\n", descKeyType(pub))
		fmt.Printf("Encoding: multibase\n")
		fmt.Printf("As DID Key: %s\n", pub.DIDKey())
		return nil
	}

	pub, err = atcrypto.ParsePublicDIDKey(s)
	if nil == err {
		fmt.Printf("Type: %s\n", descKeyType(pub))
		fmt.Printf("Encoding: DID Key\n")
		fmt.Printf("As Multibase: %s\n", pub.Multibase())
		return nil
	}
	return fmt.Errorf("unknown key encoding or type")
}

// inspectJWK parses and displays information about a JWK.
func inspectJWK(s string) error {
	// First try to parse as a private JWK (with "d" field)
	var privJWK PrivateJWK
	if err := json.Unmarshal([]byte(s), &privJWK); err != nil {
		return fmt.Errorf("invalid JWK JSON: %w", err)
	}

	if privJWK.KeyType != "EC" {
		return fmt.Errorf("unsupported JWK key type: %s", privJWK.KeyType)
	}

	// Check if this is a private key (has "d" field)
	if privJWK.D != "" {
		return inspectPrivateJWK(privJWK)
	}

	// Parse as public key using atcrypto
	pub, err := atcrypto.ParsePublicJWKBytes([]byte(s))
	if err != nil {
		return fmt.Errorf("parsing public JWK: %w", err)
	}

	fmt.Printf("Type: %s\n", descKeyType(pub))
	fmt.Printf("Encoding: JWK (public)\n")
	fmt.Printf("As DID Key: %s\n", pub.DIDKey())
	fmt.Printf("As Multibase: %s\n", pub.Multibase())
	return nil
}

// inspectPrivateJWK parses and displays information about a private JWK.
func inspectPrivateJWK(jwk PrivateJWK) error {
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return fmt.Errorf("invalid JWK 'd' parameter encoding: %w", err)
	}

	var priv atcrypto.PrivateKeyExportable
	switch jwk.Curve {
	case "P-256":
		priv, err = atcrypto.ParsePrivateBytesP256(dBytes)
		if err != nil {
			return fmt.Errorf("invalid P-256 private key: %w", err)
		}
	case "secp256k1":
		priv, err = atcrypto.ParsePrivateBytesK256(dBytes)
		if err != nil {
			return fmt.Errorf("invalid K-256 private key: %w", err)
		}
	default:
		return fmt.Errorf("unsupported JWK curve: %s", jwk.Curve)
	}

	pub, err := priv.PublicKey()
	if err != nil {
		return err
	}

	fmt.Printf("Type: %s\n", descKeyType(priv))
	fmt.Printf("Encoding: JWK (private)\n")
	fmt.Printf("As Multibase: %s\n", priv.Multibase())
	fmt.Printf("Public (DID Key): %s\n", pub.DIDKey())
	return nil
}
