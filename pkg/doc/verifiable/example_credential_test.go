/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package verifiable_test

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/aries-framework-go/pkg/doc/verifiable"
)

type UniversityDegree struct {
	Type       string `json:"type,omitempty"`
	University string `json:"university,omitempty"`
}

type UniversityDegreeSubject struct {
	ID     string           `json:"id,omitempty"`
	Name   string           `json:"name,omitempty"`
	Spouse string           `json:"spouse,omitempty"`
	Degree UniversityDegree `json:"degree,omitempty"`
}

type UniversityDegreeCredential struct {
	*verifiable.Credential

	ReferenceNumber int `json:"referenceNumber,omitempty"`
}

func (udc *UniversityDegreeCredential) MarshalJSON() ([]byte, error) {
	// todo too complex! (https://github.com/hyperledger/aries-framework-go/issues/847)
	c := udc.Credential
	cp := *c

	cp.CustomFields = map[string]interface{}{
		"referenceNumber": udc.ReferenceNumber,
	}

	return json.Marshal(&cp)
}

//nolint:gochecknoglobals
var privKey = ed25519.PrivateKey{56, 237, 176, 143, 247, 162, 167, 111, 85, 161, 158, 14, 243, 173, 144, 51, 157, 109, 155, 228, 77, 170, 238, 85, 220, 144, 158, 51, 14, 40, 153, 141, 193, 179, 12, 234, 125, 193, 60, 56, 198, 150, 80, 93, 30, 58, 14, 152, 205, 6, 50, 98, 125, 212, 65, 17, 15, 11, 230, 3, 226, 187, 7, 89} //nolint:lll

//nolint:lll
func ExampleCredential_embedding() {
	issued := time.Date(2010, time.January, 1, 19, 23, 24, 0, time.UTC)
	expired := time.Date(2020, time.January, 1, 19, 23, 24, 0, time.UTC)

	vc := &UniversityDegreeCredential{
		Credential: &verifiable.Credential{
			Context: []string{"https://www.w3.org/2018/credentials/v1", "https://www.w3.org/2018/credentials/examples/v1"},
			ID:      "http://example.edu/credentials/1872",
			Types:   []string{"VerifiableCredential", "UniversityDegreeCredential"},
			Subject: UniversityDegreeSubject{
				ID:     "did:example:ebfeb1f712ebc6f1c276e12ec21",
				Name:   "Jayden Doe",
				Spouse: "did:example:c276e12ec21ebfeb1f712ebc6f1",
				Degree: UniversityDegree{
					Type:       "BachelorDegree",
					University: "MIT",
				},
			},
			Issuer: verifiable.Issuer{
				ID:   "did:example:76e12ec712ebc6f1c221ebfeb1f",
				Name: "Example University",
			},
			Issued:  &issued,
			Expired: &expired,
			Schemas: []verifiable.TypedID{},
		},
		ReferenceNumber: 83294847,
	}

	// Marshal to JSON.
	vcBytes, err := json.Marshal(vc)
	if err != nil {
		fmt.Println("failed to marshal VC to JSON")
	}

	fmt.Println(string(vcBytes))

	// Marshal to JWS.
	jwtClaims, err := vc.JWTClaims(true)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to marshal JWT claims of VC: %w", err))
	}

	jws, err := jwtClaims.MarshalJWS(verifiable.EdDSA, privKey, "")
	if err != nil {
		fmt.Println(fmt.Errorf("failed to sign VC inside JWT: %w", err))
	}

	fmt.Println(jws)

	// Decode JWS and make sure it's coincide with JSON.
	_, vcBytesFromJWS, err := verifiable.NewCredential(
		[]byte(jws),
		verifiable.WithPublicKeyFetcher(verifiable.SingleKey(privKey.Public())))
	if err != nil {
		fmt.Println(fmt.Errorf("failed to encode VC from JWS: %w", err))
	}

	fmt.Println(string(vcBytesFromJWS))
	// todo missing referenceNumber here (https://github.com/hyperledger/aries-framework-go/issues/847)

	// Output:
	// {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSchema":[],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"referenceNumber":83294847,"type":["VerifiableCredential","UniversityDegreeCredential"]}
	// eyJhbGciOiJFZERTQSIsImtpZCI6IiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1Nzc5MDY2MDQsImlhdCI6MTI2MjM3MzgwNCwiaXNzIjoiZGlkOmV4YW1wbGU6NzZlMTJlYzcxMmViYzZmMWMyMjFlYmZlYjFmIiwianRpIjoiaHR0cDovL2V4YW1wbGUuZWR1L2NyZWRlbnRpYWxzLzE4NzIiLCJuYmYiOjEyNjIzNzM4MDQsInN1YiI6ImRpZDpleGFtcGxlOmViZmViMWY3MTJlYmM2ZjFjMjc2ZTEyZWMyMSIsInZjIjp7IkBjb250ZXh0IjpbImh0dHBzOi8vd3d3LnczLm9yZy8yMDE4L2NyZWRlbnRpYWxzL3YxIiwiaHR0cHM6Ly93d3cudzMub3JnLzIwMTgvY3JlZGVudGlhbHMvZXhhbXBsZXMvdjEiXSwiY3JlZGVudGlhbFNjaGVtYSI6W10sImNyZWRlbnRpYWxTdWJqZWN0Ijp7ImRlZ3JlZSI6eyJ0eXBlIjoiQmFjaGVsb3JEZWdyZWUiLCJ1bml2ZXJzaXR5IjoiTUlUIn0sImlkIjoiZGlkOmV4YW1wbGU6ZWJmZWIxZjcxMmViYzZmMWMyNzZlMTJlYzIxIiwibmFtZSI6IkpheWRlbiBEb2UiLCJzcG91c2UiOiJkaWQ6ZXhhbXBsZTpjMjc2ZTEyZWMyMWViZmViMWY3MTJlYmM2ZjEifSwiaXNzdWVyIjp7Im5hbWUiOiJFeGFtcGxlIFVuaXZlcnNpdHkifSwidHlwZSI6WyJWZXJpZmlhYmxlQ3JlZGVudGlhbCIsIlVuaXZlcnNpdHlEZWdyZWVDcmVkZW50aWFsIl19fQ.AHn2A2q5DL1heX3_izq_2yrsBDhoZ6BGGKhoRvhfMnMUuuOnBOdekdTg-dfUMJgipXRql_6WzBUIj4wTFehXCw
	// {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSchema":[],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"type":["VerifiableCredential","UniversityDegreeCredential"]}
}

//nolint:lll
func ExampleCredential_extraFields() {
	issued := time.Date(2010, time.January, 1, 19, 23, 24, 0, time.UTC)
	expired := time.Date(2020, time.January, 1, 19, 23, 24, 0, time.UTC)

	vc := &verifiable.Credential{
		Context: []string{"https://www.w3.org/2018/credentials/v1", "https://www.w3.org/2018/credentials/examples/v1"},
		ID:      "http://example.edu/credentials/1872",
		Types:   []string{"VerifiableCredential", "UniversityDegreeCredential"},
		Subject: UniversityDegreeSubject{
			ID:     "did:example:ebfeb1f712ebc6f1c276e12ec21",
			Name:   "Jayden Doe",
			Spouse: "did:example:c276e12ec21ebfeb1f712ebc6f1",
			Degree: UniversityDegree{
				Type:       "BachelorDegree",
				University: "MIT",
			},
		},
		Issuer: verifiable.Issuer{
			ID:   "did:example:76e12ec712ebc6f1c221ebfeb1f",
			Name: "Example University",
		},
		Issued:  &issued,
		Expired: &expired,
		Schemas: []verifiable.TypedID{},
		CustomFields: map[string]interface{}{
			"referenceNumber": 83294847,
		},
	}

	// Marshal to JSON.
	vcBytes, err := json.Marshal(vc)
	if err != nil {
		fmt.Println("failed to marshal VC to JSON")
	}

	fmt.Println(string(vcBytes))

	// Marshal to JWS.
	jwtClaims, err := vc.JWTClaims(true)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to marshal JWT claims of VC: %w", err))
	}

	jws, err := jwtClaims.MarshalJWS(verifiable.EdDSA, privKey, "")
	if err != nil {
		fmt.Println(fmt.Errorf("failed to sign VC inside JWT: %w", err))
	}

	fmt.Println(jws)

	// Decode JWS and make sure it's coincide with JSON.
	_, vcBytesFromJWS, err := verifiable.NewCredential(
		[]byte(jws),
		verifiable.WithPublicKeyFetcher(verifiable.SingleKey(privKey.Public())))
	if err != nil {
		fmt.Println(fmt.Errorf("failed to encode VC from JWS: %w", err))
	}

	fmt.Println(string(vcBytesFromJWS))

	// Output:
	// {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSchema":[],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"referenceNumber":83294847,"type":["VerifiableCredential","UniversityDegreeCredential"]}
	// eyJhbGciOiJFZERTQSIsImtpZCI6IiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1Nzc5MDY2MDQsImlhdCI6MTI2MjM3MzgwNCwiaXNzIjoiZGlkOmV4YW1wbGU6NzZlMTJlYzcxMmViYzZmMWMyMjFlYmZlYjFmIiwianRpIjoiaHR0cDovL2V4YW1wbGUuZWR1L2NyZWRlbnRpYWxzLzE4NzIiLCJuYmYiOjEyNjIzNzM4MDQsInN1YiI6ImRpZDpleGFtcGxlOmViZmViMWY3MTJlYmM2ZjFjMjc2ZTEyZWMyMSIsInZjIjp7IkBjb250ZXh0IjpbImh0dHBzOi8vd3d3LnczLm9yZy8yMDE4L2NyZWRlbnRpYWxzL3YxIiwiaHR0cHM6Ly93d3cudzMub3JnLzIwMTgvY3JlZGVudGlhbHMvZXhhbXBsZXMvdjEiXSwiY3JlZGVudGlhbFNjaGVtYSI6W10sImNyZWRlbnRpYWxTdWJqZWN0Ijp7ImRlZ3JlZSI6eyJ0eXBlIjoiQmFjaGVsb3JEZWdyZWUiLCJ1bml2ZXJzaXR5IjoiTUlUIn0sImlkIjoiZGlkOmV4YW1wbGU6ZWJmZWIxZjcxMmViYzZmMWMyNzZlMTJlYzIxIiwibmFtZSI6IkpheWRlbiBEb2UiLCJzcG91c2UiOiJkaWQ6ZXhhbXBsZTpjMjc2ZTEyZWMyMWViZmViMWY3MTJlYmM2ZjEifSwiaXNzdWVyIjp7Im5hbWUiOiJFeGFtcGxlIFVuaXZlcnNpdHkifSwicmVmZXJlbmNlTnVtYmVyIjo4LjMyOTQ4NDdlKzA3LCJ0eXBlIjpbIlZlcmlmaWFibGVDcmVkZW50aWFsIiwiVW5pdmVyc2l0eURlZ3JlZUNyZWRlbnRpYWwiXX19.auzCDgrk2TOK9BQFZHVI4p5bX1EI3CEfFNjXneC0r5fV5JE9jHY7WAIuRgKoFhNnadLKHdIekED_NrnlOEa0BA
	// {"@context":["https://www.w3.org/2018/credentials/v1","https://www.w3.org/2018/credentials/examples/v1"],"credentialSchema":[],"credentialSubject":{"degree":{"type":"BachelorDegree","university":"MIT"},"id":"did:example:ebfeb1f712ebc6f1c276e12ec21","name":"Jayden Doe","spouse":"did:example:c276e12ec21ebfeb1f712ebc6f1"},"expirationDate":"2020-01-01T19:23:24Z","id":"http://example.edu/credentials/1872","issuanceDate":"2010-01-01T19:23:24Z","issuer":{"id":"did:example:76e12ec712ebc6f1c221ebfeb1f","name":"Example University"},"referenceNumber":83294847,"type":["VerifiableCredential","UniversityDegreeCredential"]}
}
