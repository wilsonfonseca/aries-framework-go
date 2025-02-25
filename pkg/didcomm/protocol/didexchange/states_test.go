/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package didexchange

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/model"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/service"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/protocol/decorator"
	diddoc "github.com/hyperledger/aries-framework-go/pkg/doc/did"
	"github.com/hyperledger/aries-framework-go/pkg/internal/mock/didcomm/protocol"
	mockstorage "github.com/hyperledger/aries-framework-go/pkg/internal/mock/storage"
	mockvdri "github.com/hyperledger/aries-framework-go/pkg/internal/mock/vdri"
	"github.com/hyperledger/aries-framework-go/pkg/storage"
)

func TestNoopState(t *testing.T) {
	noop := &noOp{}
	require.Equal(t, "noop", noop.Name())

	t.Run("must not transition to any state", func(t *testing.T) {
		all := []state{&null{}, &invited{}, &requested{}, &responded{}, &completed{}}
		for _, s := range all {
			require.False(t, noop.CanTransitionTo(s))
		}
	})
}

// null state can transition to invited state or requested state
func TestNullState(t *testing.T) {
	null := &null{}
	require.Equal(t, "null", null.Name())
	require.False(t, null.CanTransitionTo(null))
	require.True(t, null.CanTransitionTo(&invited{}))
	require.True(t, null.CanTransitionTo(&requested{}))
	require.False(t, null.CanTransitionTo(&responded{}))
	require.False(t, null.CanTransitionTo(&completed{}))
}

// invited can only transition to requested state
func TestInvitedState(t *testing.T) {
	invited := &invited{}
	require.Equal(t, "invited", invited.Name())
	require.False(t, invited.CanTransitionTo(&null{}))
	require.False(t, invited.CanTransitionTo(invited))
	require.True(t, invited.CanTransitionTo(&requested{}))
	require.False(t, invited.CanTransitionTo(&responded{}))
	require.False(t, invited.CanTransitionTo(&completed{}))
}

// requested can only transition to responded state
func TestRequestedState(t *testing.T) {
	requested := &requested{}
	require.Equal(t, "requested", requested.Name())
	require.False(t, requested.CanTransitionTo(&null{}))
	require.False(t, requested.CanTransitionTo(&invited{}))
	require.False(t, requested.CanTransitionTo(requested))
	require.True(t, requested.CanTransitionTo(&responded{}))
	require.False(t, requested.CanTransitionTo(&completed{}))
}

// responded can only transition to completed state
func TestRespondedState(t *testing.T) {
	responded := &responded{}
	require.Equal(t, "responded", responded.Name())
	require.False(t, responded.CanTransitionTo(&null{}))
	require.False(t, responded.CanTransitionTo(&invited{}))
	require.False(t, responded.CanTransitionTo(&requested{}))
	require.False(t, responded.CanTransitionTo(responded))
	require.True(t, responded.CanTransitionTo(&completed{}))
}

// completed is an end state
func TestCompletedState(t *testing.T) {
	completed := &completed{}
	require.Equal(t, "completed", completed.Name())
	require.False(t, completed.CanTransitionTo(&null{}))
	require.False(t, completed.CanTransitionTo(&invited{}))
	require.False(t, completed.CanTransitionTo(&requested{}))
	require.False(t, completed.CanTransitionTo(&responded{}))
	require.False(t, completed.CanTransitionTo(&abandoned{}))
	require.False(t, completed.CanTransitionTo(completed))
}

func TestAbandonedState(t *testing.T) {
	abandoned := &abandoned{}
	require.Equal(t, stateNameAbandoned, abandoned.Name())
	require.False(t, abandoned.CanTransitionTo(&null{}))
	require.False(t, abandoned.CanTransitionTo(&invited{}))
	require.False(t, abandoned.CanTransitionTo(&requested{}))
	require.False(t, abandoned.CanTransitionTo(&responded{}))
	require.False(t, abandoned.CanTransitionTo(&completed{}))
	connRec, _, _, err := abandoned.ExecuteInbound(&stateMachineMsg{}, "", &context{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
	require.Nil(t, connRec)
}

func TestStateFromMsgType(t *testing.T) {
	t.Run("invited", func(t *testing.T) {
		expected := &invited{}
		actual, err := stateFromMsgType(InvitationMsgType)
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("requested", func(t *testing.T) {
		expected := &requested{}
		actual, err := stateFromMsgType(RequestMsgType)
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("responded", func(t *testing.T) {
		expected := &responded{}
		actual, err := stateFromMsgType(ResponseMsgType)
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("completed", func(t *testing.T) {
		expected := &completed{}
		actual, err := stateFromMsgType(AckMsgType)
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("invalid", func(t *testing.T) {
		actual, err := stateFromMsgType("invalid")
		require.Nil(t, actual)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unrecognized msgType: invalid")
	})
}

func TestStateFromName(t *testing.T) {
	t.Run("noop", func(t *testing.T) {
		expected := &noOp{}
		actual, err := stateFromName(expected.Name())
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("null", func(t *testing.T) {
		expected := &null{}
		actual, err := stateFromName(expected.Name())
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("invited", func(t *testing.T) {
		expected := &invited{}
		actual, err := stateFromName(expected.Name())
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("requested", func(t *testing.T) {
		expected := &requested{}
		actual, err := stateFromName(expected.Name())
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("responded", func(t *testing.T) {
		expected := &responded{}
		actual, err := stateFromName(expected.Name())
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("completed", func(t *testing.T) {
		expected := &completed{}
		actual, err := stateFromName(expected.Name())
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("abandoned", func(t *testing.T) {
		expected := &abandoned{}
		actual, err := stateFromName(expected.Name())
		require.NoError(t, err)
		require.Equal(t, expected.Name(), actual.Name())
	})
	t.Run("undefined", func(t *testing.T) {
		actual, err := stateFromName("undefined")
		require.Nil(t, actual)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state name")
	})
}

// noOp.ExecuteInbound() returns nil, error
func TestNoOpState_Execute(t *testing.T) {
	_, followup, _, err := (&noOp{}).ExecuteInbound(&stateMachineMsg{}, "", &context{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot execute no-op")
	require.Nil(t, followup)
}

// null.ExecuteInbound() is a no-op
func TestNullState_Execute(t *testing.T) {
	_, followup, _, err := (&null{}).ExecuteInbound(&stateMachineMsg{}, "", &context{})
	require.NoError(t, err)
	require.IsType(t, &noOp{}, followup)
}

func TestInvitedState_Execute(t *testing.T) {
	t.Run("rejects msgs other than invitations", func(t *testing.T) {
		others := []string{RequestMsgType, ResponseMsgType, AckMsgType}
		for _, o := range others {
			_, _, _, err := (&invited{}).ExecuteInbound(&stateMachineMsg{header: &service.Header{Type: o}},
				"", &context{})
			require.Error(t, err)
			require.Contains(t, err.Error(), "illegal msg type")
		}
	})
	t.Run("followup to 'requested' on inbound invitations", func(t *testing.T) {
		invitationPayloadBytes, err := json.Marshal(&Invitation{
			Type:            InvitationMsgType,
			ID:              randomString(),
			Label:           "Bob",
			RecipientKeys:   []string{"8HH5gYEeNc3z7PYXmd54d4x6qAfCNrqQqEB3nS7Zfu7K"},
			ServiceEndpoint: "https://localhost:8090",
			RoutingKeys:     []string{"8HH5gYEeNc3z7PYXmd54d4x6qAfCNrqQqEB3nS7Zfu7K"},
		})
		require.NoError(t, err)
		connRec, followup, _, err := (&invited{}).ExecuteInbound(
			&stateMachineMsg{
				header:     &service.Header{Type: InvitationMsgType},
				payload:    invitationPayloadBytes,
				connRecord: &ConnectionRecord{},
			},
			"",
			&context{})
		require.NoError(t, err)
		require.Equal(t, &requested{}, followup)
		require.NotNil(t, connRec)
	})
}
func TestRequestedState_Execute(t *testing.T) {
	prov, store := getProvider()
	// Alice receives an invitation from Bob
	invitationPayloadBytes, err := json.Marshal(&Invitation{
		Type:            InvitationMsgType,
		ID:              randomString(),
		Label:           "Bob",
		RecipientKeys:   []string{"8HH5gYEeNc3z7PYXmd54d4x6qAfCNrqQqEB3nS7Zfu7K"},
		ServiceEndpoint: "https://localhost:8090",
		RoutingKeys:     []string{"8HH5gYEeNc3z7PYXmd54d4x6qAfCNrqQqEB3nS7Zfu7K"},
	})
	require.NoError(t, err)
	t.Run("rejects messages other than invitations or requests", func(t *testing.T) {
		others := []string{ResponseMsgType, AckMsgType}
		for _, o := range others {
			_, _, _, e := (&requested{}).ExecuteInbound(&stateMachineMsg{header: &service.Header{Type: o}}, "", &context{})
			require.Error(t, e)
			require.Contains(t, e.Error(), "illegal msg type")
		}
	})
	t.Run("handle inbound invitations", func(t *testing.T) {
		ctx := getContext(prov, store)
		msg, err := service.NewDIDCommMsg(invitationPayloadBytes)
		require.NoError(t, err)
		// nolint: govet
		thid, err := threadID(msg)
		require.NoError(t, err)
		connRec, _, _, e := (&requested{}).ExecuteInbound(&stateMachineMsg{
			header:     msg.Header,
			payload:    msg.Payload,
			connRecord: &ConnectionRecord{},
		}, thid, ctx)
		require.NoError(t, e)
		require.NotNil(t, connRec.MyDID)
	})
	t.Run("inbound request unmarshalling error", func(t *testing.T) {
		_, followup, _, err := (&requested{}).ExecuteInbound(&stateMachineMsg{
			header:  &service.Header{Type: InvitationMsgType},
			payload: nil,
		}, "", &context{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "JSON unmarshalling of invitation")
		require.Nil(t, followup)
	})
	t.Run("create DID error", func(t *testing.T) {
		ctx2 := &context{outboundDispatcher: prov.OutboundDispatcher(),
			vdriRegistry: &mockvdri.MockVDRIRegistry{CreateErr: fmt.Errorf("create DID error")}}
		didDoc, err := ctx2.vdriRegistry.Create(testMethod)
		require.Error(t, err)
		require.Contains(t, err.Error(), "create DID error")
		require.Nil(t, didDoc)
	})
	t.Run("handle inbound invitation public key error", func(t *testing.T) {
		connRec := &ConnectionRecord{
			State:        (&requested{}).Name(),
			ThreadID:     "test",
			ConnectionID: "123",
		}
		didDoc := getMockDID()
		didDoc.Service[0].RecipientKeys = []string{"invalid"}
		ctx2 := &context{outboundDispatcher: prov.OutboundDispatcher(),
			vdriRegistry: &mockvdri.MockVDRIRegistry{CreateValue: didDoc},
			signer:       &mockSigner{}}
		_, followup, _, err := (&requested{}).ExecuteInbound(&stateMachineMsg{
			header:     &service.Header{Type: InvitationMsgType},
			payload:    invitationPayloadBytes,
			connRecord: connRec,
		}, "", ctx2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "key not found in DID document")
		require.Nil(t, followup)
	})
}
func TestRespondedState_Execute(t *testing.T) {
	prov, store := getProvider()
	ctx := getContext(prov, store)
	request, err := createRequest(ctx)
	require.NoError(t, err)
	requestPayloadBytes, err := json.Marshal(request)
	require.NoError(t, err)
	response, err := createResponse(request, ctx)
	require.NoError(t, err)
	responsePayloadBytes, err := json.Marshal(response)
	require.NoError(t, err)

	t.Run("rejects messages other than requests and responses", func(t *testing.T) {
		others := []string{InvitationMsgType, AckMsgType}
		for _, o := range others {
			_, _, _, e := (&responded{}).ExecuteInbound(&stateMachineMsg{header: &service.Header{Type: o}}, "", &context{})
			require.Error(t, e)
			require.Contains(t, e.Error(), "illegal msg type")
		}
	})
	t.Run("no followup for inbound requests", func(t *testing.T) {
		connRec, followup, _, e := (&responded{}).ExecuteInbound(&stateMachineMsg{
			header:     &service.Header{Type: RequestMsgType},
			payload:    requestPayloadBytes,
			connRecord: &ConnectionRecord{},
		}, "", ctx)
		require.NoError(t, e)
		require.NotNil(t, connRec)
		require.IsType(t, &noOp{}, followup)
	})
	t.Run("followup to 'completed' on inbound responses", func(t *testing.T) {
		connRec := &ConnectionRecord{
			State:        (&responded{}).Name(),
			ThreadID:     request.ID,
			ConnectionID: "123",
		}
		err = ctx.connectionStore.saveConnectionRecord(connRec)
		require.NoError(t, err)
		err = ctx.connectionStore.saveNSThreadID(request.ID, findNameSpace(ResponseMsgType), connRec.ConnectionID)
		require.NoError(t, err)
		connRec, followup, _, e := (&responded{}).ExecuteInbound(
			&stateMachineMsg{
				header:     &service.Header{Type: ResponseMsgType},
				payload:    responsePayloadBytes,
				connRecord: connRec,
			}, "", ctx)
		require.NoError(t, e)
		require.NotNil(t, connRec)
		require.Equal(t, (&completed{}).Name(), followup.Name())
	})
	t.Run("handle inbound request public key error", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service[0].RecipientKeys = []string{"invalid"}
		ctx2 := &context{outboundDispatcher: prov.OutboundDispatcher(),
			vdriRegistry: &mockvdri.MockVDRIRegistry{CreateValue: didDoc}, signer: &mockSigner{},
			connectionStore: NewConnectionRecorder(store, store)}
		_, followup, _, err := (&responded{}).ExecuteInbound(&stateMachineMsg{
			header:     &service.Header{Type: RequestMsgType},
			payload:    requestPayloadBytes,
			connRecord: &ConnectionRecord{},
		}, "", ctx2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "key not found in DID document")
		require.Nil(t, followup)
	})
	t.Run("handle inbound request unmarshalling error", func(t *testing.T) {
		_, followup, _, err := (&responded{}).ExecuteInbound(&stateMachineMsg{
			header: &service.Header{Type: RequestMsgType},
		}, "", &context{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "JSON unmarshalling of request")
		require.Nil(t, followup)
	})
}
func TestAbandonedState_Execute(t *testing.T) {
	t.Run("execute abandon state", func(t *testing.T) {
		connRec, _, _, err := (&abandoned{}).ExecuteInbound(&stateMachineMsg{
			header: &service.Header{Type: ResponseMsgType}}, "", &context{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "not implemented")
		require.Nil(t, connRec)
	})
}

// completed is an end state
func TestCompletedState_Execute(t *testing.T) {
	pubKey, privKey := generateKeyPair()
	_, store := getProvider()
	ctx := &context{signer: &mockSigner{privateKey: privKey},
		connectionStore: NewConnectionRecorder(store, store)}
	newDIDDoc := createDIDDocWithKey(pubKey)
	connection := &Connection{
		DID:    newDIDDoc.ID,
		DIDDoc: newDIDDoc,
	}
	invitation, err := createMockInvitation(pubKey, ctx)
	require.NoError(t, err)
	connectionSignature, err := ctx.prepareConnectionSignature(connection, invitation.ID)
	require.NoError(t, err)

	response := &Response{
		Type:                ResponseMsgType,
		ID:                  randomString(),
		ConnectionSignature: connectionSignature,
		Thread: &decorator.Thread{
			ID: "test",
		},
	}
	responsePayloadBytes, err := json.Marshal(response)
	require.NoError(t, err)

	t.Run("no followup for inbound responses", func(t *testing.T) {
		connRec := &ConnectionRecord{
			State:         (&responded{}).Name(),
			ThreadID:      response.Thread.ID,
			ConnectionID:  "123",
			MyDID:         "did:peer:123456789abcdefghi#inbox",
			Namespace:     myNSPrefix,
			InvitationID:  invitation.ID,
			RecipientKeys: []string{pubKey},
		}
		err = ctx.connectionStore.saveNewConnectionRecord(connRec)
		require.NoError(t, err)
		ctx.vdriRegistry = &mockvdri.MockVDRIRegistry{ResolveValue: getMockDID()}
		require.NoError(t, err)
		_, followup, _, e := (&completed{}).ExecuteInbound(&stateMachineMsg{
			header:     &service.Header{Type: ResponseMsgType},
			payload:    responsePayloadBytes,
			connRecord: connRec,
		}, "", ctx)
		require.NoError(t, e)
		require.IsType(t, &noOp{}, followup)
	})
	t.Run("no followup for inbound acks", func(t *testing.T) {
		connRec := &ConnectionRecord{
			State:         (&responded{}).Name(),
			ThreadID:      response.Thread.ID,
			ConnectionID:  "123",
			RecipientKeys: []string{pubKey},
		}
		err = ctx.connectionStore.saveConnectionRecord(connRec)
		require.NoError(t, err)
		err = ctx.connectionStore.saveNSThreadID(response.Thread.ID, findNameSpace(AckMsgType), connRec.ConnectionID)
		require.NoError(t, err)
		ack := &model.Ack{
			Type:   AckMsgType,
			ID:     randomString(),
			Status: ackStatusOK,
			Thread: &decorator.Thread{
				ID: response.Thread.ID,
			}}
		ackPayloadBytes, e := json.Marshal(ack)
		require.NoError(t, e)
		_, followup, _, e := (&completed{}).ExecuteInbound(&stateMachineMsg{
			header:  &service.Header{Type: AckMsgType},
			payload: ackPayloadBytes,
		}, "", ctx)
		require.NoError(t, e)
		require.IsType(t, &noOp{}, followup)
	})
	t.Run("rejects messages other than responses and acks", func(t *testing.T) {
		others := []string{InvitationMsgType, RequestMsgType}
		for _, o := range others {
			_, _, _, err = (&completed{}).ExecuteInbound(&stateMachineMsg{header: &service.Header{Type: o}}, "", &context{})
			require.Error(t, err)
			require.Contains(t, err.Error(), "illegal msg type")
		}
	})
	t.Run("no followup for inbound responses unmarshalling error", func(t *testing.T) {
		_, followup, _, err := (&completed{}).ExecuteInbound(&stateMachineMsg{
			header: &service.Header{Type: ResponseMsgType},
		}, "", &context{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "JSON unmarshalling of response")
		require.Nil(t, followup)
	})
	t.Run("execute inbound handle inbound response  error", func(t *testing.T) {
		response.ConnectionSignature = &ConnectionSignature{}
		responsePayloadBytes, err := json.Marshal(response)
		require.NoError(t, err)
		_, followup, _, err := (&completed{}).ExecuteInbound(&stateMachineMsg{
			header:  &service.Header{Type: ResponseMsgType},
			payload: responsePayloadBytes,
		}, "", ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "handle inbound response")
		require.Nil(t, followup)
	})
}

func TestVerifySignature(t *testing.T) {
	pubKey, privKey := generateKeyPair()
	_, store := getProvider()
	ctx := &context{signer: &mockSigner{privateKey: privKey},
		connectionStore: NewConnectionRecorder(nil, store)}
	newDIDDoc := createDIDDocWithKey(pubKey)
	connection := &Connection{
		DID:    newDIDDoc.ID,
		DIDDoc: newDIDDoc,
	}
	invitation, err := createMockInvitation(pubKey, ctx)
	require.NoError(t, err)

	t.Run("signature verified", func(t *testing.T) {
		connectionSignature, err := ctx.prepareConnectionSignature(connection, invitation.ID)
		require.NoError(t, err)
		con, err := verifySignature(connectionSignature, invitation.RecipientKeys[0])
		require.NoError(t, err)
		require.NotNil(t, con)
		require.Equal(t, newDIDDoc.ID, con.DID)
	})
	t.Run("missing/invalid signature data", func(t *testing.T) {
		con, err := verifySignature(&ConnectionSignature{}, invitation.RecipientKeys[0])
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing or invalid signature data")
		require.Nil(t, con)
	})
	t.Run("decode signature data error", func(t *testing.T) {
		connectionSignature, err := ctx.prepareConnectionSignature(connection, invitation.ID)
		require.NoError(t, err)

		connectionSignature.SignedData = "invalid-signed-data"
		con, err := verifySignature(connectionSignature, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "decode signature data: illegal base64 data")
		require.Nil(t, con)
	})
	t.Run("decode signature error", func(t *testing.T) {
		connectionSignature, err := ctx.prepareConnectionSignature(connection, invitation.ID)
		require.NoError(t, err)

		connectionSignature.Signature = "invalid-signature"
		con, err := verifySignature(connectionSignature, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "decode signature: illegal base64 data")
		require.Nil(t, con)
	})
	t.Run("decode verification key error ", func(t *testing.T) {
		connectionSignature, err := ctx.prepareConnectionSignature(connection, invitation.ID)
		require.NoError(t, err)

		con, err := verifySignature(connectionSignature, "invalid-key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "verify signature: ed25519: bad public key length")
		require.Nil(t, con)
	})
	t.Run("verify signature error", func(t *testing.T) {
		connectionSignature, err := ctx.prepareConnectionSignature(connection, invitation.ID)
		require.NoError(t, err)

		// generate different key and assign it to signature verification key
		pubKey2, _ := generateKeyPair()
		con, err := verifySignature(connectionSignature, pubKey2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "signature doesn't match")
		require.Nil(t, con)
	})
	t.Run("connection unmarshal error", func(t *testing.T) {
		connAttributeBytes := []byte("{hello world}")

		now := getEpochTime()
		timestamp := strconv.FormatInt(now, 10)
		prefix := append([]byte(timestamp), signatureDataDelimiter)
		concatenateSignData := append(prefix, connAttributeBytes...)

		signature, err := ctx.signer.SignMessage(concatenateSignData, pubKey)
		require.NoError(t, err)

		cs := &ConnectionSignature{
			Type:       "https://didcomm.org/signature/1.0/ed25519Sha512_single",
			SignedData: base64.URLEncoding.EncodeToString(concatenateSignData),
			SignVerKey: base64.URLEncoding.EncodeToString(base58.Decode(pubKey)),
			Signature:  base64.URLEncoding.EncodeToString(signature),
		}

		con, err := verifySignature(cs, invitation.RecipientKeys[0])
		require.Error(t, err)
		require.Contains(t, err.Error(), "JSON unmarshalling of connection")
		require.Nil(t, con)
	})
	t.Run("missing connection attribute bytes", func(t *testing.T) {
		now := getEpochTime()
		timestamp := strconv.FormatInt(now, 10)
		prefix := append([]byte(timestamp), signatureDataDelimiter)

		signature, err := ctx.signer.SignMessage(prefix, pubKey)
		require.NoError(t, err)

		cs := &ConnectionSignature{
			Type:       "https://didcomm.org/signature/1.0/ed25519Sha512_single",
			SignedData: base64.URLEncoding.EncodeToString(prefix),
			SignVerKey: base64.URLEncoding.EncodeToString(base58.Decode(pubKey)),
			Signature:  base64.URLEncoding.EncodeToString(signature),
		}

		con, err := verifySignature(cs, invitation.RecipientKeys[0])
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing connection attribute bytes")
		require.Nil(t, con)
	})
}

func TestPrepareConnectionSignature(t *testing.T) {
	prov, store := getProvider()
	ctx := getContext(prov, store)
	pubKey, privKey := generateKeyPair()
	invitation, err := createMockInvitation(pubKey, ctx)
	require.NoError(t, err)
	newDidDoc, err := ctx.vdriRegistry.Create(testMethod)
	require.NoError(t, err)

	connection := &Connection{
		DID:    newDidDoc.ID,
		DIDDoc: newDidDoc,
	}

	t.Run("prepare connection signature", func(t *testing.T) {
		connectionSignature, err := ctx.prepareConnectionSignature(connection, invitation.ID)
		require.NoError(t, err)
		require.NotNil(t, connectionSignature)
		sigData, err := base64.URLEncoding.DecodeString(connectionSignature.SignedData)
		require.NoError(t, err)
		connBytes := bytes.SplitAfter(sigData, []byte(string(signatureDataDelimiter)))
		sigDataConnection := &Connection{}
		err = json.Unmarshal(connBytes[1], sigDataConnection)
		require.NoError(t, err)
		require.Equal(t, connection.DID, sigDataConnection.DID)
	})
	t.Run("implicit invitation with DID - success", func(t *testing.T) {
		ctx2 := &context{outboundDispatcher: prov.OutboundDispatcher(),
			vdriRegistry:    &mockvdri.MockVDRIRegistry{ResolveValue: newDidDoc},
			signer:          &mockSigner{privateKey: privKey},
			connectionStore: NewConnectionRecorder(store, store),
		}
		connectionSignature, err := ctx2.prepareConnectionSignature(connection, newDidDoc.ID)
		require.NoError(t, err)
		require.NotNil(t, connectionSignature)
		sigData, err := base64.URLEncoding.DecodeString(connectionSignature.SignedData)
		require.NoError(t, err)
		connBytes := bytes.SplitAfter(sigData, []byte(string(signatureDataDelimiter)))
		sigDataConnection := &Connection{}
		err = json.Unmarshal(connBytes[1], sigDataConnection)
		require.NoError(t, err)
		require.Equal(t, connection.DID, sigDataConnection.DID)
	})
	t.Run("implicit invitation with DID - recipient key error", func(t *testing.T) {
		newDidDoc.PublicKey = nil
		ctx2 := &context{outboundDispatcher: prov.OutboundDispatcher(),
			vdriRegistry:    &mockvdri.MockVDRIRegistry{ResolveValue: newDidDoc},
			signer:          &mockSigner{privateKey: privKey},
			connectionStore: NewConnectionRecorder(store, store),
		}
		connectionSignature, err := ctx2.prepareConnectionSignature(connection, newDidDoc.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "key not found in DID document")
		require.Nil(t, connectionSignature)
	})
	t.Run("prepare connection signature get invitation", func(t *testing.T) {
		connectionSignature, err := ctx.prepareConnectionSignature(connection, "test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "get invitation for signature: data not found")
		require.Nil(t, connectionSignature)
	})
	t.Run("prepare connection signature get invitation", func(t *testing.T) {
		inv := &Invitation{
			Type: InvitationMsgType,
			ID:   randomString(),
			DID:  "test",
		}
		err := ctx.connectionStore.SaveInvitation(invitation)
		require.NoError(t, err)
		connectionSignature, err := ctx.prepareConnectionSignature(connection, inv.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "get invitation for signature: data not found")
		require.Nil(t, connectionSignature)
	})
	t.Run("prepare connection signature error", func(t *testing.T) {
		ctx := &context{signer: &mockSigner{err: errors.New("sign error")},
			connectionStore: NewConnectionRecorder(nil, store)}
		connection := &Connection{
			DIDDoc: getMockDID(),
		}
		connectionSignature, err := ctx.prepareConnectionSignature(connection, invitation.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "sign error")
		require.Nil(t, connectionSignature)
	})
}

func TestPrepareDestination(t *testing.T) {
	t.Run("successfully prepared destination", func(t *testing.T) {
		dest, err := prepareDestination(getMockDID())
		require.NoError(t, err)
		require.NotNil(t, dest)
		require.Equal(t, dest.ServiceEndpoint, "https://localhost:8090")
	})

	t.Run("error while getting service", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service = nil

		dest, err := prepareDestination(didDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "service not found in DID document: did-communication")
		require.Nil(t, dest)
	})

	t.Run("error while getting recipient keys from did doc", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service[0].RecipientKeys = []string{}

		recipientKeys, err := getRecipientKeys(didDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing recipient keys in did-communication service")
		require.Nil(t, recipientKeys)
	})
}

func TestNewRequestFromInvitation(t *testing.T) {
	invitation := &Invitation{
		Type:            InvitationMsgType,
		ID:              randomString(),
		Label:           "Bob",
		RecipientKeys:   []string{"8HH5gYEeNc3z7PYXmd54d4x6qAfCNrqQqEB3nS7Zfu7K"},
		ServiceEndpoint: "https://localhost:8090",
		RoutingKeys:     []string{"8HH5gYEeNc3z7PYXmd54d4x6qAfCNrqQqEB3nS7Zfu7K"},
	}

	t.Run("successful new request from invitation", func(t *testing.T) {
		prov, store := getProvider()
		ctx := getContext(prov, store)
		invitationBytes, err := json.Marshal(invitation)
		require.NoError(t, err)
		thid, err := threadID(&service.DIDCommMsg{
			Header:  &service.Header{Type: InvitationMsgType},
			Payload: invitationBytes,
		})
		require.NoError(t, err)
		_, connRec, err := ctx.handleInboundInvitation(invitation, thid, &options{}, &ConnectionRecord{})
		require.NoError(t, err)
		require.NotNil(t, connRec.MyDID)
	})
	t.Run("successful response to invitation with public did", func(t *testing.T) {
		doc := createDIDDoc()
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveValue: doc}}

		invitationBytes, err := json.Marshal(invitation)
		require.NoError(t, err)
		thid, err := threadID(&service.DIDCommMsg{
			Header:  &service.Header{Type: InvitationMsgType},
			Payload: invitationBytes,
		})
		require.NoError(t, err)
		_, connRec, err := ctx.handleInboundInvitation(invitation, thid, &options{publicDID: doc.ID}, &ConnectionRecord{})
		require.NoError(t, err)
		require.NotNil(t, connRec.MyDID)
		require.Equal(t, connRec.MyDID, doc.ID)
	})
	t.Run("unsuccessful new request from invitation ", func(t *testing.T) {
		prov := protocol.MockProvider{}
		ctx := &context{outboundDispatcher: prov.OutboundDispatcher(),
			vdriRegistry: &mockvdri.MockVDRIRegistry{CreateErr: fmt.Errorf("create DID error")}}
		invitationBytes, err := json.Marshal(&Invitation{})
		require.NoError(t, err)
		thid, err := threadID(&service.DIDCommMsg{
			Header:  &service.Header{Type: InvitationMsgType},
			Payload: invitationBytes,
		})
		require.NoError(t, err)
		_, connRec, err := ctx.handleInboundInvitation(invitation, thid, &options{}, &ConnectionRecord{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "create DID error")
		require.Nil(t, connRec)
	})
}

func TestNewResponseFromRequest(t *testing.T) {
	prov, store := getProvider()

	t.Run("successful new response from request", func(t *testing.T) {
		ctx := getContext(prov, store)
		request, err := createRequest(ctx)
		require.NoError(t, err)
		_, connRec, err := ctx.handleInboundRequest(request, &options{}, &ConnectionRecord{})
		require.NoError(t, err)
		require.NotNil(t, connRec.MyDID)
		require.NotNil(t, connRec.TheirDID)
	})
	t.Run("unsuccessful new response from request due to create did error", func(t *testing.T) {
		didDoc := getMockDID()
		ctx := &context{
			vdriRegistry: &mockvdri.MockVDRIRegistry{CreateErr: fmt.Errorf("create DID error"), ResolveValue: getMockDID()}}
		request := &Request{Connection: &Connection{DID: didDoc.ID, DIDDoc: didDoc}}
		_, connRec, err := ctx.handleInboundRequest(request, &options{}, &ConnectionRecord{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "create DID error")
		require.Nil(t, connRec)
	})
	t.Run("unsuccessful new response from request due to sign error", func(t *testing.T) {
		ctx := &context{vdriRegistry: &mockvdri.MockVDRIRegistry{CreateValue: getMockDID()},
			signer:          &mockSigner{err: errors.New("sign error")},
			connectionStore: NewConnectionRecorder(nil, store)}
		request, err := createRequest(ctx)
		require.NoError(t, err)
		_, connRec, err := ctx.handleInboundRequest(request, &options{}, &ConnectionRecord{})

		require.Error(t, err)
		require.Contains(t, err.Error(), "sign error")
		require.Nil(t, connRec)
	})
	t.Run("unsuccessful new response from request due to resolve public did from request error", func(t *testing.T) {
		ctx := &context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveErr: errors.New("resolver error")}}
		request := &Request{Connection: &Connection{DID: "did:sidetree:abc"}}
		_, _, err := ctx.handleInboundRequest(request, &options{}, &ConnectionRecord{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "resolver error")
	})
}

func TestHandleInboundResponse(t *testing.T) {
	pubKey, _ := generateKeyPair()
	prov, store := getProvider()
	ctx := getContext(prov, store)
	_, err := createMockInvitation(pubKey, ctx)
	require.NoError(t, err)
	request, err := createRequest(ctx)
	require.NoError(t, err)

	t.Run("handle inbound responses get connection record error", func(t *testing.T) {
		response := &Response{Thread: &decorator.Thread{ID: "test"}}
		_, connRec, err := ctx.handleInboundResponse(response)
		require.Error(t, err)
		require.Contains(t, err.Error(), "get connection record")
		require.Nil(t, connRec)
	})
	t.Run("handle inbound responses get connection record error", func(t *testing.T) {
		response := &Response{Thread: &decorator.Thread{ID: ""}}
		_, connRec, err := ctx.handleInboundResponse(response)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty bytes")
		require.Nil(t, connRec)
	})
	t.Run("handle inbound responses missing signature data", func(t *testing.T) {
		resp, err := saveMockConnectionRecord(request, ctx)
		require.NoError(t, err)
		resp.ConnectionSignature = &ConnectionSignature{}
		_, connRec, e := ctx.handleInboundResponse(resp)
		require.Error(t, e)
		require.Contains(t, e.Error(), "missing or invalid signature data")
		require.Nil(t, connRec)
	})
}
func TestGetInvitationRecipientKey(t *testing.T) {
	prov, _ := getProvider()
	ctx := getContext(prov, nil)

	t.Run("successfully getting invitation recipient key", func(t *testing.T) {
		invitation := &Invitation{
			Type:            InvitationMsgType,
			ID:              randomString(),
			Label:           "Bob",
			RecipientKeys:   []string{"test"},
			ServiceEndpoint: "http://alice.agent.example.com:8081",
		}
		recKey, err := ctx.getInvitationRecipientKey(invitation)
		require.NoError(t, err)
		require.Equal(t, invitation.RecipientKeys[0], recKey)
	})
	t.Run("failed to get invitation recipient key", func(t *testing.T) {
		doc := getMockDID()
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveValue: doc}}
		invitation := &Invitation{
			Type: InvitationMsgType,
			ID:   randomString(),
			DID:  doc.ID,
		}
		recKey, err := ctx.getInvitationRecipientKey(invitation)
		require.NoError(t, err)
		require.Equal(t, string(doc.PublicKey[0].Value), recKey)
	})
	t.Run("failed to get invitation recipient key", func(t *testing.T) {
		invitation := &Invitation{
			Type: InvitationMsgType,
			ID:   randomString(),
			DID:  "test",
		}
		_, err := ctx.getInvitationRecipientKey(invitation)
		require.Error(t, err)
		require.Contains(t, err.Error(), "get invitation recipient key: not found")
	})
}

func TestGetPublicKey(t *testing.T) {
	t.Run("successfully getting public key by id", func(t *testing.T) {
		prov := protocol.MockProvider{}
		ctx := getContext(prov, nil)
		newDidDoc, err := ctx.vdriRegistry.Create(testMethod)
		require.NoError(t, err)
		pubkey, err := getPublicKey(newDidDoc.PublicKey[0].ID, newDidDoc)
		require.NoError(t, err)
		require.NotNil(t, pubkey)
	})
	t.Run("failed to get public key", func(t *testing.T) {
		prov := protocol.MockProvider{}
		ctx := getContext(prov, nil)
		newDidDoc, err := ctx.vdriRegistry.Create(testMethod)
		require.NoError(t, err)
		pubkey, err := getPublicKey("invalid-key", newDidDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "key not found in DID document: invalid-key")
		require.Nil(t, pubkey)
	})
}

func TestGetDidCommService(t *testing.T) {
	t.Run("successfully getting did-communication service", func(t *testing.T) {
		didDoc := getMockDID()

		s, err := getDidCommService(didDoc)
		require.NoError(t, err)
		require.Equal(t, "did-communication", s.Type)
		require.Equal(t, uint(0), s.Priority)
	})

	t.Run("error due to missing service", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service = nil

		s, err := getDidCommService(didDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "service not found in DID document: did-communication")
		require.Nil(t, s)
	})

	t.Run("error due to missing did-communication service", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service[0].Type = "some-type"
		didDoc.Service[1].Type = "other-type"

		s, err := getDidCommService(didDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "service not found in DID document: did-communication")
		require.Nil(t, s)
	})
}

func TestGetRecipientKeys(t *testing.T) {
	t.Run("successfully getting recipient keys", func(t *testing.T) {
		didDoc := getMockDID()

		recipientKeys, err := getRecipientKeys(didDoc)
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientKeys))
	})

	t.Run("error due to missing did-communication service", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service = nil

		recipientKeys, err := getRecipientKeys(didDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "service not found in DID document: did-communication")
		require.Nil(t, recipientKeys)
	})

	t.Run("error due to missing recipient keys in did-communication service", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service[0].RecipientKeys = []string{}

		recipientKeys, err := getRecipientKeys(didDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing recipient keys in did-communication service")
		require.Nil(t, recipientKeys)
	})

	t.Run("error due to missing public key in did doc", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service[0].RecipientKeys = []string{"invalid"}

		recipientKeys, err := getRecipientKeys(didDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "key not found in DID document: invalid")
		require.Nil(t, recipientKeys)
	})

	t.Run("error due to unsupported key types", func(t *testing.T) {
		didDoc := getMockDID()
		didDoc.Service[0].RecipientKeys = []string{didDoc.PublicKey[0].ID}

		recipientKeys, err := getRecipientKeys(didDoc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "recipient keys in did-communication service not supported")
		require.Nil(t, recipientKeys)
	})
}

func TestGetDIDDocAndConnection(t *testing.T) {
	t.Run("successfully getting did doc and connection for public did", func(t *testing.T) {
		doc := createDIDDoc()
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveValue: doc}}
		didDoc, conn, err := ctx.getDIDDocAndConnection(doc.ID)
		require.NoError(t, err)
		require.NotNil(t, didDoc)
		require.NotNil(t, conn)
		require.Equal(t, didDoc.ID, conn.DID)
	})
	t.Run("error getting public did doc from resolver", func(t *testing.T) {
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveErr: errors.New("resolver error")}}
		didDoc, conn, err := ctx.getDIDDocAndConnection("did-id")
		require.Error(t, err)
		require.Contains(t, err.Error(), "resolver error")
		require.Nil(t, didDoc)
		require.Nil(t, conn)
	})
	t.Run("error creating peer did", func(t *testing.T) {
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{CreateErr: errors.New("creator error")}}
		didDoc, conn, err := ctx.getDIDDocAndConnection("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "creator error")
		require.Nil(t, didDoc)
		require.Nil(t, conn)
	})
	t.Run("successfully created peer did", func(t *testing.T) {
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{CreateValue: getMockDID()}}
		didDoc, conn, err := ctx.getDIDDocAndConnection("")
		require.NoError(t, err)
		require.NotNil(t, didDoc)
		require.NotNil(t, conn)
		require.Equal(t, didDoc.ID, conn.DID)
	})
}

func TestGetDestinationFromDID(t *testing.T) {
	doc := createDIDDoc()

	t.Run("successfully getting destination from public DID", func(t *testing.T) {
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveValue: doc}}
		destination, err := ctx.getDestinationFromDID(doc.ID)
		require.NoError(t, err)
		require.NotNil(t, destination)
	})
	t.Run("test public key not found", func(t *testing.T) {
		doc.PublicKey = nil
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveValue: doc}}
		destination, err := ctx.getDestinationFromDID(doc.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "key not found in DID document")
		require.Nil(t, destination)
	})
	t.Run("test service not found", func(t *testing.T) {
		doc2 := createDIDDoc()
		doc2.Service = nil
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveValue: doc2}}
		destination, err := ctx.getDestinationFromDID(doc2.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "service not found in DID document: did-communication")
		require.Nil(t, destination)
	})
	t.Run("get destination by invitation", func(t *testing.T) {
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveValue: createDIDDoc()}}
		invitation := &Invitation{DID: "test"}
		destination, err := ctx.getDestination(invitation)
		require.NoError(t, err)
		require.NotNil(t, destination)
	})
	t.Run("test did document not found", func(t *testing.T) {
		ctx := context{vdriRegistry: &mockvdri.MockVDRIRegistry{ResolveErr: errors.New("resolver error")}}
		destination, err := ctx.getDestinationFromDID(doc.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "resolver error")
		require.Nil(t, destination)
	})
}

type mockSigner struct {
	privateKey []byte
	err        error
}

func (s *mockSigner) SignMessage(message []byte, fromVerKey string) ([]byte, error) {
	if s.privateKey != nil {
		return ed25519.Sign(s.privateKey, message), nil
	}

	return nil, s.err
}

func createDIDDoc() *diddoc.Doc {
	pubKey, _ := generateKeyPair()
	return createDIDDocWithKey(pubKey)
}

func createDIDDocWithKey(pub string) *diddoc.Doc {
	const (
		didFormat    = "did:%s:%s"
		didPKID      = "%s#keys-%d"
		didServiceID = "%s#endpoint-%d"
		method       = "test"
	)

	id := fmt.Sprintf(didFormat, method, pub[:16])
	pubKeyID := fmt.Sprintf(didPKID, id, 1)
	pubKey := diddoc.PublicKey{
		ID:         pubKeyID,
		Type:       "Ed25519VerificationKey2018",
		Controller: id,
		Value:      []byte(pub),
	}
	services := []diddoc.Service{
		{
			ID:              fmt.Sprintf(didServiceID, id, 1),
			Type:            "did-communication",
			ServiceEndpoint: "http://localhost:58416",
			Priority:        0,
			RecipientKeys:   []string{pubKeyID},
		},
	}
	createdTime := time.Now()
	didDoc := &diddoc.Doc{
		Context:   []string{diddoc.Context},
		ID:        id,
		PublicKey: []diddoc.PublicKey{pubKey},
		Service:   services,
		Created:   &createdTime,
		Updated:   &createdTime,
	}

	return didDoc
}

func getProvider() (protocol.MockProvider, *mockstorage.MockStore) {
	return protocol.MockProvider{}, &mockstorage.MockStore{Store: make(map[string][]byte)}
}

func getContext(prov protocol.MockProvider, store storage.Store) *context {
	pubKey, privKey := generateKeyPair()

	return &context{outboundDispatcher: prov.OutboundDispatcher(),
		vdriRegistry:    &mockvdri.MockVDRIRegistry{CreateValue: createDIDDocWithKey(pubKey)},
		signer:          &mockSigner{privateKey: privKey},
		connectionStore: NewConnectionRecorder(store, store),
	}
}

func createRequest(ctx *context) (*Request, error) {
	pubKey, _ := generateKeyPair()

	invitation, err := createMockInvitation(pubKey, ctx)
	if err != nil {
		return nil, err
	}

	newDidDoc := createDIDDocWithKey(pubKey)
	// Prepare did-exchange inbound request
	request := &Request{
		Type:  RequestMsgType,
		ID:    randomString(),
		Label: "Bob",
		Thread: &decorator.Thread{
			PID: invitation.ID,
		},

		Connection: &Connection{
			DID:    newDidDoc.ID,
			DIDDoc: newDidDoc,
		},
	}

	return request, nil
}

func createResponse(request *Request, ctx *context) (*Response, error) {
	didDoc, err := ctx.vdriRegistry.Create(testMethod)
	if err != nil {
		return nil, err
	}

	connection := &Connection{
		DID:    didDoc.ID,
		DIDDoc: didDoc,
	}

	connectionSignature, err := ctx.prepareConnectionSignature(connection, request.Thread.PID)
	if err != nil {
		return nil, err
	}

	response := &Response{
		Type: ResponseMsgType,
		ID:   randomString(),
		Thread: &decorator.Thread{
			ID: request.ID,
		},
		ConnectionSignature: connectionSignature,
	}

	return response, nil
}

func saveMockConnectionRecord(request *Request, ctx *context) (*Response, error) {
	response, err := createResponse(request, ctx)

	if err != nil {
		return nil, err
	}

	pubKey, _ := generateKeyPair()
	connRec := &ConnectionRecord{
		State:         (&responded{}).Name(),
		ThreadID:      response.Thread.ID,
		ConnectionID:  "123",
		InvitationID:  request.Thread.PID,
		RecipientKeys: []string{pubKey},
	}
	err = ctx.connectionStore.saveConnectionRecord(connRec)

	if err != nil {
		return nil, err
	}

	err = ctx.connectionStore.saveNSThreadID(response.Thread.ID, findNameSpace(ResponseMsgType),
		connRec.ConnectionID)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func generateKeyPair() (string, []byte) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	return base58.Encode(pubKey[:]), privKey
}

func createMockInvitation(pubKey string, ctx *context) (*Invitation, error) {
	invitation := &Invitation{
		Type:            InvitationMsgType,
		ID:              randomString(),
		Label:           "Bob",
		RecipientKeys:   []string{pubKey},
		ServiceEndpoint: "http://alice.agent.example.com:8081",
	}
	err := ctx.connectionStore.SaveInvitation(invitation)

	if err != nil {
		return nil, err
	}

	return invitation, nil
}
