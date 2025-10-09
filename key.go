package main

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/atcrypto"

	"github.com/urfave/cli/v3"
)

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
	var priv atcrypto.PrivateKey
	var privMultibase string
	switch cmd.String("type") {
	case "", "P-256", "p256", "ES256", "secp256r1":
		sec, err := atcrypto.GeneratePrivateKeyP256()
		if err != nil {
			return err
		}
		privMultibase = sec.Multibase()
		priv = sec
	case "K-256", "k256", "ES256K", "secp256k1":
		sec, err := atcrypto.GeneratePrivateKeyK256()
		if err != nil {
			return err
		}
		privMultibase = sec.Multibase()
		priv = sec
	default:
		return fmt.Errorf("unknown key type: %s", cmd.String("type"))
	}
	if cmd.Bool("terse") {
		fmt.Println(privMultibase)
		return nil
	}
	pub, err := priv.PublicKey()
	if err != nil {
		return err
	}
	fmt.Printf("Key Type: %s\n", descKeyType(priv))
	fmt.Printf("Secret Key (Multibase Syntax): save this securely (eg, add to password manager)\n\t%s\n", privMultibase)
	fmt.Printf("Public Key (DID Key Syntax): share or publish this (eg, in DID document)\n\t%s\n", pub.DIDKey())
	return nil
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
