// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package security provides functions for group authentication and HMAC encryption/decryption
package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	config "github.com/TeamFairmont/boltshared/config"
)

// GetKeyFromGroup takes a group name and a pointer to array of groups.
// It returns the matching hmac key
func GetKeyFromGroup(group string, groups *[]config.SecurityGroups) (key string, err error) {
	for _, thisGroup := range *groups {
		if thisGroup.Name == group {
			return thisGroup.Hmackey, nil
		}
	}
	// Group name not found.  The group name to find doesn't exist in the array of groups.  Return an error.
	return "Error", errors.New("Security error- Invalid group name")
}

// signString receives a string to sign and the key to sign with.
// It returns []byte representing the hmac signed string.
func signString(stringToSign []byte, sharedSecretKey []byte) []byte {
	h := hmac.New(sha512.New, sharedSecretKey)
	h.Write(stringToSign)
	return h.Sum(nil)
}

// checkMAC reports whether messageMAC is a valid HMAC tag for message.
// Use hmac.Equal to compare MACs in order to avoid timing side-channels
func checkMAC(message, messageMAC, key []byte) bool {
	expectedMAC := []byte(hex.EncodeToString(signString(message, key)))
	return hmac.Equal(messageMAC, expectedMAC)
}

//verifyTime checks to make sure the encoded hmac we received is within the timeout threshold (default is 30 seconds).
//The timeout is set in etc/bolt/config.json > security > verifyTimeout
//It receives []byte of json that gets unmarshaled, checks that the timestamp in the unmarshaled data is less than 30 seconds old, and returns the decoded payload if it is.
func verifyTime(decodedJSON []byte, verifyTimeout int64) (map[string]string, error) {
	payload := make(map[string]string)
	err := json.Unmarshal(decodedJSON, &payload)
	if err != nil {
		return nil, err
	}

	time64, err := strconv.ParseInt(payload["timestamp"], 10, 64)
	if err != nil {
		return nil, err
	}

	// Only process the request if the timestamp is within +-verifyTimeout seconds (default 30 seconds) of the current time
	timeNow := time.Now().Unix()
	if (timeNow-time64 > verifyTimeout) || (timeNow-time64 < -verifyTimeout) {
		var buffer bytes.Buffer
		buffer.WriteString("Security error- Invalid timestamp (")
		buffer.WriteString(strconv.FormatInt(timeNow-time64, 10))
		buffer.WriteString(") outside of verifyTime (+/- ")
		buffer.WriteString(strconv.FormatInt(verifyTimeout, 10))
		buffer.WriteString(")")
		return nil, errors.New(buffer.String())
	}

	return payload, nil
}

//AuthenticateGroup takes a group name, the key they've submitted, and Config's groups+keys.
//If the key & group received match a group within Config, return true (authentic).
//If no match is found, return false.
func AuthenticateGroup(group string, key string, groups *[]config.SecurityGroups) (authenticated bool) {
	for _, thisGroup := range *groups {
		if thisGroup.Name == group && thisGroup.Hmackey == key {
			return true
		}
	}
	return false
}

//EncodeHMAC takes
// * An hmac key
// * The raw string they would like to encode
// * The current timestamp (a payload with a timestamp of greater than or less than the timeout threshold (default is 30 seconds) will not be decrypted.)
//It encodes the message using the key.
//It returns an encrypted message []byte or an error.
//A message must be decoded within the timeout threshold (default of 30 seconds) of when it was encoded.
func EncodeHMAC(key string, rawmessage string, timestamp string) (encodedmessage []byte, err error) {

	// Create a json object with the message to encode and a timestamp
	payload := make(map[string]string)
	payload["timestamp"] = timestamp
	payload["message"] = rawmessage

	// Marshal the message into []byte
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Create the signed string.
	signature := signString(jsonBytes, []byte(key))

	// Combine the encoded message with the key signature
	jsonStr := map[string]string{
		"data":      base64.URLEncoding.EncodeToString(jsonBytes),
		"signature": base64.URLEncoding.EncodeToString([]byte(hex.EncodeToString(signature))),
	}

	// Marshal the encoded message and signature.
	encodedmessage, err = json.Marshal(jsonStr)
	if err != nil {
		return nil, err
	}

	return encodedmessage, nil
}

//DecodeHMAC takes an hmac key and the ciphered message []byte they would like to decode.
//It matches the key with the group, then encodes the message using that key.
//It returns the decoded message as a string or an error.
//A message must be decoded within the timeout threshold (default is 30 seconds) of when it was encoded.
func DecodeHMAC(key string, encodedmessage []byte, verifyTimeout int64) (decodedMessage string, err error) {

	type encodedStruct struct {
		Data      string
		Signature string
	}
	var enc encodedStruct
	err = json.Unmarshal(encodedmessage, &enc)
	if err != nil {
		return "Error Unmarshalling encodedStruct", err
	}

	decodedJSON, err := base64.URLEncoding.DecodeString(enc.Data)
	if err != nil {
		decodedJSON, err = base64.StdEncoding.DecodeString(enc.Data)
		if err != nil {
			return "Error decoding Data", err
		}
	}

	decodedSignature, err := base64.URLEncoding.DecodeString(enc.Signature)
	// decodedSignature, err := base64.StdEncoding.DecodeString(enc.Signature)
	if err != nil {
		return "Error decoding Signature", err
	}

	// Sign the received message to verify that our signature matches the signature that was sent with the message
	stringVerified := checkMAC(decodedJSON, decodedSignature, []byte(key))
	if !stringVerified {
		err := errors.New("Security error - Invalid signature")
		// return "Error verifying hmac", err
		return string(decodedSignature), err
	}

	// Check that the timestamp is within the timeout threshold (default is 30 seconds)
	payload, err := verifyTime(decodedJSON, verifyTimeout)
	if err != nil {
		return "Error verifying time", err
	}

	// The timestamp in the payload is less than the timeout threshold.  Return the decoded message.
	decodedMessage = payload["message"]
	return decodedMessage, nil
}
