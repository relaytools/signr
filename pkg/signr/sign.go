package signr

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/minio/sha256-simd"
	"github.com/mleku/ec/schnorr"
	secp "github.com/mleku/ec/secp"
	"github.com/mleku/signr/pkg/nostr"
)

// Sign some data using a key.
//
// By default the signature includes all of the prefix text that was also used
// to generate the hash to sign on, for namespacing, which follows the format:
//
//	signr_0_SHA256_SCHNORR_
//
// which is always present. After this can be a custom string, that is sanitised
// and all whitespaces between its characters are changed to hyphens, to provide
// namespacing for a custom protocol.
//
// After this by default there is a random 64 bit nonce to ensure the signature
// is not applied to a repeating hash for the given protocol.
//
// in all cases following this first 4 sections and optional nonce and custom
// namespace, is the public key of the secret key used to make the signature.
// This prevents any collisions being found between signatures generated by
// different secret keys.
//
// Function arguments:
//
// args are one or two, being a filename, a hex string of the hash of a file or
// blob of data, the second, optional args element is the name of the key in the
// keychain to use.
//
// pass is the password to decrypt the default or specified private key being
// used.
//
// custom is an extra custom namespace string that is cleaned of whitespaces and
// spaces replaced with hyphens, and inserted between the usual first 4
// namespace fields and the nonce and/or public key.
//
// asHex specifies to return the signature as 128 raw hex characters, rather
// than bech32 encoding with the HRP 'sig'.
//
// sigOnly specifies to return only the signature and not the standard prefixed
// form. This is implicitly used if the first 'args' parameter is a 64 character
// long hash in hex format.
func (s *Signr) Sign(args []string, pass, custom string,
	asHex, sigOnly bool) (sigStr string, err error) {

	signingKey := s.DefaultKey
	filename := args[0]
	switch {
	case len(args) < 1:
		err = fmt.Errorf(
			"ERROR: at minimum a file to be signed needs to  be specified")
		return
	case len(args) > 1:
		var keySlice []string
		keySlice, err = s.GetKeyPairNames()
		if err != nil {
			err = fmt.Errorf("ERROR: '%s'", err)
			return
		}
		var found bool
		for _, k := range keySlice {
			if k == args[1] {
				found, signingKey = true, k
			}
		}
		if !found {
			err = fmt.Errorf("'%s' key not found", args[1])
			return
		}
	}
	signingStrings := GetDefaultSigningStrings()
	signingStrings = s.AddCustom(signingStrings, custom)
	var skipRandomness bool
	if sigOnly || asHex {
		skipRandomness = true
	}
	// if the command line contains a raw hash we assume that a simple
	// signature on this is intended. it will still use the namespacing, the
	// pubkey and any custom string, but not the nonce. it is assumed that
	// the protocol generating the hash has accounted for sufficient
	// entropy.
	var sum []byte
	if len(filename) == 64 {
		sum, err = hex.DecodeString(filename)
		if err == nil {
			skipRandomness = true
		}
	}
	// hash the file
	if len(sum) == 0 {
		if sum, err = HashFile(filename); err != nil {
			err = fmt.Errorf("error while generating hash on file/input: %s",
				err)
			return
		}
	}
	if !skipRandomness {
		// add the signature nonce
		var nonce string
		if nonce, err = s.GetNonceHex(); err != nil {
			err = fmt.Errorf("ERROR: getting nonce: %s", err)
			return
		}
		signingStrings = append(signingStrings, nonce)
	}
	// add the public key. This must always be present as it isolates
	// the namespace of even intra-protocol signing.
	var pkb []byte
	if pkb, err = s.ReadFile(signingKey + "." + PubExt); err != nil {
		err = fmt.Errorf("error while reading file: %s", err)
		return
	}
	// the keychain stores secrets as hex but the pubkeys in nostr npub.
	// nsec keys are not encrypted.
	signingStrings = append(signingStrings, strings.TrimSpace(string(pkb)))
	// append the checksum.
	signingStrings = append(signingStrings, hex.EncodeToString(sum))
	// construct the signing material.
	message := strings.Join(signingStrings, "_")
	s.Log("signing on message: %s\n", message)
	messageHash := sha256.Sum256([]byte(message))
	var key *secp.SecretKey
	if key, err = s.GetKey(signingKey, pass); err != nil {
		return
	}
	var sig *schnorr.Signature
	if sig, err = schnorr.Sign(key, messageHash[:]); err != nil {
		err = fmt.Errorf("ERROR: while signing: '%s'", err)
		return
	}
	if skipRandomness {
		if asHex {
			sigStr = hex.EncodeToString(sig.Serialize())
		} else {
			if sigStr, err = nostr.EncodeSignature(sig); err != nil {
				err = fmt.Errorf("error while formatting signature: %s",
					err)
				return
			}
		}
		return
	}
	// a standard signr signature with the signature in place of the hash of the
	// last element of the signing material.
	if sigStr, err = FormatSig(signingStrings, sig); err != nil {
		err = fmt.Errorf("ERROR: %s", err)
		return
	}
	return
}
