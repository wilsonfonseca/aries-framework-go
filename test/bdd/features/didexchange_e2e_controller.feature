#
# Copyright SecureKey Technologies Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# Reference : https://github.com/hyperledger/aries-rfcs/tree/master/features/0023-did-exchange

@all
@didexchange_e2e_controller
Feature: Decentralized Identifier(DID) exchange between the agents using controller API

  Scenario: did exchange e2e flow using controller api
    Given "Alice" agent is running on "localhost" port "8081" with controller "http://localhost:8082" and webhook "http://localhost:8083"
    And "Bob" agent is running on "localhost" port "9081" with controller "http://localhost:9082" and webhook "http://localhost:9083"
    And   "Alice" creates invitation through controller with label "alice-agent"
    And   "Bob" receives invitation from "Alice" through controller
    And   "Bob" approves exchange invitation
    And   "Alice" approves exchange request
    And   "Alice" waits for post state event "completed" to webhook
    And   "Bob" waits for post state event "completed" to webhook
    And   "Alice" retrieves connection record through controller and validates that connection state is "completed"
    And   "Bob" retrieves connection record through controller and validates that connection state is "completed"
