/*
Copyright SecureKey Technologies Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package verifiable

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPresentationFromJWS(t *testing.T) {
	vpBytes := []byte(validPresentation)

	keyFetcher := createPresKeyFetcher(t)

	t.Run("Decoding presentation from JWS", func(t *testing.T) {
		jws := createPresJWS(t, vpBytes, false)
		vpFromJWT, err := NewPresentation(jws, WithPresPublicKeyFetcher(keyFetcher))
		require.NoError(t, err)

		vp, err := NewPresentation(vpBytes)
		require.NoError(t, err)

		require.Equal(t, vp, vpFromJWT)
	})

	t.Run("Decoding presentation from JWS with minimized fields of \"vp\" claim", func(t *testing.T) {
		jws := createPresJWS(t, vpBytes, true)
		vpFromJWT, err := NewPresentation(jws, WithPresPublicKeyFetcher(keyFetcher))
		require.NoError(t, err)

		vp, err := NewPresentation(vpBytes)
		require.NoError(t, err)

		require.Equal(t, vp, vpFromJWT)
	})

	t.Run("Failed JWT signature verification of presentation", func(t *testing.T) {
		jws := createPresJWS(t, vpBytes, true)
		_, err := NewPresentation(
			jws,
			// passing issuers's key, while expecting issuer one
			WithPresPublicKeyFetcher(func(issuerID, keyID string) (interface{}, error) {
				publicKey, err := readPublicKey(filepath.Join(certPrefix, "issuer_public.pem"))
				require.NoError(t, err)
				require.NotNil(t, publicKey)

				return publicKey, nil
			}))

		require.Error(t, err)
		require.Contains(t, err.Error(), "decoding of Verifiable Presentation from JWS")
	})

	t.Run("Failed public key fetching", func(t *testing.T) {
		jws := createPresJWS(t, vpBytes, true)
		_, err := NewPresentation(
			jws,
			WithPresPublicKeyFetcher(func(issuerID, keyID string) (interface{}, error) {
				return nil, errors.New("test: public key is not found")
			}))

		require.Error(t, err)
		require.Contains(t, err.Error(), "get public key for JWT signature verification")
	})

	t.Run("Not defined public key fetcher", func(t *testing.T) {
		_, err := NewPresentation(createPresJWS(t, vpBytes, true))

		require.Error(t, err)
		require.Contains(t, err.Error(), "public key fetcher is not defined")
	})
}

func TestNewPresentationFromJWS_EdDSA(t *testing.T) {
	vpBytes := []byte(validPresentation)

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	vp, err := NewPresentation(vpBytes)
	require.NoError(t, err)

	// marshal presentation into JWS using EdDSA (Ed25519 signature algorithm).
	jwtClaims := vp.JWTClaims([]string{}, false)
	vpJWSStr, err := jwtClaims.MarshalJWS(EdDSA, privKey, vp.Holder+"#keys-"+keyID)
	require.NoError(t, err)

	// unmarshal presentation from JWS
	vpFromJWS, err := NewPresentation(
		[]byte(vpJWSStr),
		WithPresPublicKeyFetcher(SingleKey(pubKey)))
	require.NoError(t, err)

	// unmarshalled presentation must be the same as original one
	require.Equal(t, vp, vpFromJWS)
}

func TestNewPresentationFromUnsecuredJWT(t *testing.T) {
	vpBytes := []byte(validPresentation)

	t.Run("Decoding presentation from unsecured JWT", func(t *testing.T) {
		vpFromJWT, err := NewPresentation(createPresUnsecuredJWT(t, vpBytes, false))

		require.NoError(t, err)

		vp, err := NewPresentation(vpBytes)
		require.NoError(t, err)

		require.Equal(t, vp, vpFromJWT)
	})

	t.Run("Decoding presentation from unsecured JWT with minimized fields of \"vp\" claim", func(t *testing.T) {
		vpFromJWT, err := NewPresentation(createPresUnsecuredJWT(t, vpBytes, true))

		require.NoError(t, err)

		vp, err := NewPresentation(vpBytes)
		require.NoError(t, err)

		require.Equal(t, vp, vpFromJWT)
	})
}

func createPresJWS(t *testing.T, vpBytes []byte, minimize bool) []byte {
	vp, err := NewPresentation(vpBytes)
	require.NoError(t, err)

	privateKey, err := readPrivateKey(filepath.Join(certPrefix, "holder_private.pem"))
	require.NoError(t, err)

	jwtClaims := vp.JWTClaims([]string{}, minimize)
	vpJWT, err := jwtClaims.MarshalJWS(RS256, privateKey, vp.Holder+"#keys-"+keyID)
	require.NoError(t, err)

	return []byte(vpJWT)
}

func createPresKeyFetcher(t *testing.T) func(issuerID string, keyID string) (interface{}, error) {
	return func(issuerID, keyID string) (interface{}, error) {
		require.Equal(t, "did:example:ebfeb1f712ebc6f1c276e12ec21", issuerID)
		require.Equal(t, "did:example:ebfeb1f712ebc6f1c276e12ec21#keys-1", keyID)

		publicKey, err := readPublicKey(filepath.Join(certPrefix, "holder_public.pem"))
		require.NoError(t, err)
		require.NotNil(t, publicKey)

		return publicKey, nil
	}
}

func createPresUnsecuredJWT(t *testing.T, cred []byte, minimize bool) []byte {
	vp, err := NewPresentation(cred)
	require.NoError(t, err)

	jwtClaims := vp.JWTClaims([]string{}, minimize)

	vpJWT, err := jwtClaims.MarshalUnsecuredJWT()
	require.NoError(t, err)

	return []byte(vpJWT)
}
