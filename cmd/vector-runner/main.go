package main

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/talosprotocol/talos-sdk-go/pkg/talos/canonical"
	"github.com/talosprotocol/talos-sdk-go/pkg/talos/wallet"
	"golang.org/x/crypto/chacha20poly1305"
)

type summary struct {
	passed  int
	failed  int
	skipped int
}

func (s *summary) merge(other summary) {
	s.passed += other.passed
	s.failed += other.failed
	s.skipped += other.skipped
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/vector-runner <vectors.json>")
		os.Exit(1)
	}

	result, err := runPath(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("Conformance summary: passed=%d failed=%d skipped=%d\n", result.passed, result.failed, result.skipped)
	if result.failed > 0 {
		os.Exit(1)
	}
}

func runPath(path string) (summary, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return summary{}, fmt.Errorf("read %s: %w", path, err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return summary{}, fmt.Errorf("decode %s: %w", path, err)
	}

	if isReleaseSet(payload) {
		return runReleaseSet(path, payload)
	}

	switch filepath.Base(path) {
	case "canonical_json.json":
		return runCanonicalVectors(path, payload)
	case "signing_verify.json":
		return runSigningVectors(path, payload)
	case "capability_verify.json":
		return runCapabilityVectors(path, payload)
	case "frame_codec.json":
		return runFrameVectors(path, payload)
	case "mcp_sign_verify.json":
		return runMCPVectors(path, payload)
	case "header_canonical_bytes.json":
		return runHeaderVectors(path, payload)
	case "kdf_rk_step.json":
		return runKdfRootVector(payload)
	case "kdf_ck_step.json":
		return runKdfChainVector(payload)
	case "v1_1_0_roundtrip.json":
		return runRatchetRoundtripVector(payload)
	default:
		fmt.Printf("SKIP %s (no Go conformance handler yet)\n", filepath.Base(path))
		return summary{skipped: 1}, nil
	}
}

func isReleaseSet(payload map[string]any) bool {
	vectors, ok := payload["vectors"].([]any)
	if !ok || len(vectors) == 0 {
		return false
	}
	for _, candidate := range vectors {
		if _, ok := candidate.(string); !ok {
			return false
		}
	}
	return true
}

func runReleaseSet(path string, payload map[string]any) (summary, error) {
	var result summary
	baseDir := filepath.Dir(path)
	for _, candidate := range payload["vectors"].([]any) {
		vectorPath := filepath.Join(baseDir, candidate.(string))
		sub, err := runPath(vectorPath)
		if err != nil {
			return result, err
		}
		result.merge(sub)
	}
	return result, nil
}

func runCanonicalVectors(path string, payload map[string]any) (summary, error) {
	var result summary
	vectors, ok := payload["vectors"].([]any)
	if !ok {
		return result, fmt.Errorf("%s missing vectors array", path)
	}

	for _, candidate := range vectors {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed vector", path)
		}

		testID := stringField(vec, "test_id", "unknown")
		if err := checkCanonicalVector(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	return result, nil
}

func checkCanonicalVector(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	expected, ok := vec["expected"].(map[string]any)
	if !ok {
		return errors.New("missing expected object")
	}

	var payload any
	switch {
	case hasKey(inputs, "unordered"):
		payload = inputs["unordered"]
	case hasKey(inputs, "value"):
		payload = inputs["value"]
	case hasKey(inputs, "pretty_printed"):
		raw, ok := inputs["pretty_printed"].(string)
		if !ok {
			return errors.New("pretty_printed must be a string")
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return fmt.Errorf("parse pretty_printed JSON: %w", err)
		}
	default:
		return errors.New("unsupported canonical vector shape")
	}

	canonicalBytes, err := canonical.Marshal(payload)
	if err != nil {
		return fmt.Errorf("canonical marshal failed: %w", err)
	}
	canonicalString := string(canonicalBytes)

	if want, ok := expected["canonical"].(string); ok && canonicalString != want {
		return fmt.Errorf("canonical mismatch: got %s want %s", canonicalString, want)
	}
	if want, ok := expected["canonical_number"].(string); ok && canonicalString != want {
		return fmt.Errorf("canonical number mismatch: got %s want %s", canonicalString, want)
	}
	return nil
}

func runSigningVectors(path string, payload map[string]any) (summary, error) {
	var result summary

	for _, candidate := range listField(payload, "vectors") {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed positive vector", path)
		}
		testID := stringField(vec, "test_id", "unknown")
		if err := checkSigningVector(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	for _, candidate := range listField(payload, "negative_cases") {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed negative vector", path)
		}
		testID := stringField(vec, "test_id", "unknown-negative")
		if err := checkSigningNegative(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	return result, nil
}

func checkSigningVector(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	expected, ok := vec["expected"].(map[string]any)
	if !ok {
		return errors.New("missing expected object")
	}

	seedHex, ok := inputs["seed_hex"].(string)
	if !ok {
		return errors.New("seed_hex must be a string")
	}
	seedBytes, err := hex.DecodeString(seedHex)
	if err != nil {
		return fmt.Errorf("decode seed_hex: %w", err)
	}

	w, err := wallet.FromSeed(seedBytes, "")
	if err != nil {
		return fmt.Errorf("wallet from seed: %w", err)
	}
	message := []byte(stringField(inputs, "message_utf8", ""))
	signature := w.Sign(message)

	if want, ok := expected["public_key_hex"].(string); ok {
		if got := hex.EncodeToString(w.PublicKey()); got != want {
			return fmt.Errorf("public_key_hex mismatch: got %s want %s", got, want)
		}
	}
	if want, ok := expected["did"].(string); ok && w.DID() != want {
		return fmt.Errorf("did mismatch: got %s want %s", w.DID(), want)
	}
	if want, ok := expected["signature_base64url"].(string); ok {
		got := base64.RawURLEncoding.EncodeToString(signature)
		if got != want {
			return fmt.Errorf("signature mismatch: got %s want %s", got, want)
		}
	}
	if want, ok := expected["signature_length"].(float64); ok && len(signature) != int(want) {
		return fmt.Errorf("signature length mismatch: got %d want %d", len(signature), int(want))
	}
	if want, ok := expected["verify"].(bool); ok {
		if got := wallet.Verify(w.PublicKey(), message, signature); got != want {
			return fmt.Errorf("verify mismatch: got %t want %t", got, want)
		}
	}

	return nil
}

func checkSigningNegative(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	testID := stringField(vec, "test_id", "unknown-negative")

	switch testID {
	case "invalid_seed_length":
		seedHex, _ := inputs["seed_hex"].(string)
		seedBytes, err := hex.DecodeString(seedHex)
		if err != nil {
			return fmt.Errorf("decode seed_hex: %w", err)
		}
		if _, err := wallet.FromSeed(seedBytes, ""); err == nil {
			return errors.New("expected invalid seed error")
		}
		return nil
	case "verify_wrong_key":
		message := []byte(stringField(inputs, "message_utf8", ""))
		signature, err := base64.RawURLEncoding.DecodeString(stringField(inputs, "signature_base64url", ""))
		if err != nil {
			return fmt.Errorf("decode signature: %w", err)
		}
		publicKey, err := hex.DecodeString(stringField(inputs, "wrong_public_key_hex", ""))
		if err != nil {
			return fmt.Errorf("decode wrong_public_key_hex: %w", err)
		}
		if wallet.Verify(publicKey, message, signature) {
			return errors.New("verification unexpectedly succeeded with wrong key")
		}
		return nil
	case "verify_tampered_message":
		seedBytes, err := hex.DecodeString(stringField(inputs, "seed_hex", ""))
		if err != nil {
			return fmt.Errorf("decode seed_hex: %w", err)
		}
		w, err := wallet.FromSeed(seedBytes, "")
		if err != nil {
			return fmt.Errorf("wallet from seed: %w", err)
		}
		signature := w.Sign([]byte(stringField(inputs, "original_message", "")))
		if wallet.Verify(w.PublicKey(), []byte(stringField(inputs, "tampered_message", "")), signature) {
			return errors.New("verification unexpectedly succeeded for tampered message")
		}
		return nil
	default:
		return fmt.Errorf("unsupported negative signing vector %s", testID)
	}
}

func runCapabilityVectors(path string, payload map[string]any) (summary, error) {
	var result summary

	for _, candidate := range listField(payload, "vectors") {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed positive vector", path)
		}
		testID := stringField(vec, "test_id", "unknown")
		if err := checkCapabilityVector(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	for _, candidate := range listField(payload, "negative_cases") {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed negative vector", path)
		}
		testID := stringField(vec, "test_id", "unknown-negative")
		if err := checkCapabilityNegative(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	return result, nil
}

func checkCapabilityVector(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	expected, ok := vec["expected"].(map[string]any)
	if !ok {
		return errors.New("missing expected object")
	}

	capability, publicKey, exp, err := makeSignedCapability(inputs)
	if err != nil {
		return err
	}

	if want, ok := expected["verify"].(bool); ok {
		got, reason, err := verifyCapability(capability, publicKey, (defaultCapabilityIssuedAt+exp)/2)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("verify mismatch: got %t want %t (reason=%s)", got, want, reason)
		}
	}

	for key, value := range expected {
		want, ok := value.(bool)
		if !ok || len(key) <= len("authorize_fast_") || key[:len("authorize_fast_")] != "authorize_fast_" {
			continue
		}
		parts := splitAuthorizeKey(key)
		if len(parts) != 2 {
			return fmt.Errorf("invalid authorize key %s", key)
		}
		got := capabilityAuthorize(capability["scope"], parts[0], parts[1])
		if got != want {
			return fmt.Errorf("authorize mismatch for %s/%s: got %t want %t", parts[0], parts[1], got, want)
		}
	}

	return nil
}

func checkCapabilityNegative(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	testID := stringField(vec, "test_id", "unknown-negative")

	switch testID {
	case "capability_expired":
		capability, publicKey, exp, err := makeSignedCapability(map[string]any{
			"issuer_seed_hex": seedHexOne,
			"subject_did":     defaultCapabilitySubject,
			"scope":           defaultCapabilityScope(),
			"exp":             numberField(inputs, "token_with_exp", 0),
		})
		if err != nil {
			return err
		}
		got, reason, err := verifyCapability(capability, publicKey, exp+1)
		if err != nil {
			return err
		}
		if got || reason != "expired" {
			return fmt.Errorf("expected expired capability failure, got verify=%t reason=%s", got, reason)
		}
		return nil
	case "capability_bad_signature":
		capability, publicKey, exp, err := makeSignedCapability(map[string]any{
			"issuer_seed_hex": seedHexOne,
			"subject_did":     defaultCapabilitySubject,
			"scope":           defaultCapabilityScope(),
			"exp":             float64(defaultCapabilityExpiry),
		})
		if err != nil {
			return err
		}
		signature, err := base64.RawURLEncoding.DecodeString(stringField(capability, "sig", ""))
		if err != nil {
			return fmt.Errorf("decode signature: %w", err)
		}
		if len(signature) == 0 {
			return errors.New("signature unexpectedly empty")
		}
		signature[0] ^= 0xff
		capability["sig"] = base64.RawURLEncoding.EncodeToString(signature)
		got, reason, err := verifyCapability(capability, publicKey, (defaultCapabilityIssuedAt+exp)/2)
		if err != nil {
			return err
		}
		if got || reason != "signature" {
			return fmt.Errorf("expected signature failure, got verify=%t reason=%s", got, reason)
		}
		return nil
	case "capability_wrong_issuer":
		capability, _, exp, err := makeSignedCapability(map[string]any{
			"issuer_seed_hex": seedHexOne,
			"subject_did":     defaultCapabilitySubject,
			"scope":           defaultCapabilityScope(),
			"exp":             float64(defaultCapabilityExpiry),
		})
		if err != nil {
			return err
		}
		otherSeed, err := hex.DecodeString(seedHexTwo)
		if err != nil {
			return fmt.Errorf("decode alternate seed: %w", err)
		}
		otherWallet, err := wallet.FromSeed(otherSeed, "")
		if err != nil {
			return fmt.Errorf("wallet from alternate seed: %w", err)
		}
		got, _, err := verifyCapability(capability, otherWallet.PublicKey(), (defaultCapabilityIssuedAt+exp)/2)
		if err != nil {
			return err
		}
		if got {
			return errors.New("verification unexpectedly succeeded with wrong issuer")
		}
		return nil
	default:
		return fmt.Errorf("unsupported negative capability vector %s", testID)
	}
}

func runFrameVectors(path string, payload map[string]any) (summary, error) {
	var result summary

	for _, candidate := range listField(payload, "vectors") {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed positive vector", path)
		}
		testID := stringField(vec, "test_id", "unknown")
		if err := checkFrameVector(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	for _, candidate := range listField(payload, "negative_cases") {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed negative vector", path)
		}
		testID := stringField(vec, "test_id", "unknown-negative")
		if err := checkFrameNegative(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	return result, nil
}

func checkFrameVector(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	expected, ok := vec["expected"].(map[string]any)
	if !ok {
		return errors.New("missing expected object")
	}

	if frameType, ok := inputs["frame_type"].(string); ok {
		payload := []byte(stringField(inputs, "payload_utf8", ""))
		encoded, err := encodeFrame(frameType, payload, 1, 0)
		if err != nil {
			return err
		}
		if want, ok := expected["encoded_base64url"].(string); ok && encoded != want {
			return fmt.Errorf("encoded_base64url mismatch: got %s want %s", encoded, want)
		}
	}

	if encoded, ok := inputs["encoded_base64url"].(string); ok {
		frame, err := decodeFrameBase64URL(encoded)
		if err != nil {
			return err
		}
		if want, ok := expected["frame_type"].(string); ok && stringField(frame, "type", "") != want {
			return fmt.Errorf("frame_type mismatch: got %s want %s", stringField(frame, "type", ""), want)
		}
		if want, ok := expected["version"].(float64); ok && int(numberField(frame, "version", 1)) != int(want) {
			return fmt.Errorf("version mismatch: got %d want %d", int(numberField(frame, "version", 1)), int(want))
		}
		if want, ok := expected["flags"].(float64); ok && int(numberField(frame, "flags", 0)) != int(want) {
			return fmt.Errorf("flags mismatch: got %d want %d", int(numberField(frame, "flags", 0)), int(want))
		}
	}

	return nil
}

func checkFrameNegative(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	testID := stringField(vec, "test_id", "unknown-negative")

	switch testID {
	case "decode_truncated":
		if _, err := decodeFrameBase64URL(stringField(inputs, "encoded_base64url", "")); err == nil {
			return errors.New("expected truncated frame error")
		}
		return nil
	case "decode_invalid_type":
		if _, err := decodeFrameBase64URL(stringField(inputs, "encoded_base64url", "")); err == nil {
			return errors.New("expected invalid frame type error")
		}
		return nil
	case "decode_garbage":
		raw, err := hex.DecodeString(stringField(inputs, "encoded_hex", ""))
		if err != nil {
			return fmt.Errorf("decode encoded_hex: %w", err)
		}
		if _, err := decodeFrameRaw(raw); err == nil {
			return errors.New("expected garbage frame decode error")
		}
		return nil
	default:
		return fmt.Errorf("unsupported negative frame vector %s", testID)
	}
}

func runMCPVectors(path string, payload map[string]any) (summary, error) {
	var result summary

	for _, candidate := range listField(payload, "vectors") {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed positive vector", path)
		}
		testID := stringField(vec, "test_id", "unknown")
		if err := checkMCPVector(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	for _, candidate := range listField(payload, "negative_cases") {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed negative vector", path)
		}
		testID := stringField(vec, "test_id", "unknown-negative")
		if err := checkMCPNegative(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	return result, nil
}

func checkMCPVector(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	expected, ok := vec["expected"].(map[string]any)
	if !ok {
		return errors.New("missing expected object")
	}

	seedBytes, err := hex.DecodeString(stringField(inputs, "signer_seed_hex", ""))
	if err != nil {
		return fmt.Errorf("decode signer_seed_hex: %w", err)
	}
	w, err := wallet.FromSeed(seedBytes, "")
	if err != nil {
		return fmt.Errorf("wallet from signer seed: %w", err)
	}

	payload := map[string]any{
		"request":        inputs["request"],
		"session_id":     stringField(inputs, "session_id", ""),
		"correlation_id": stringField(inputs, "correlation_id", ""),
		"tool":           stringField(inputs, "tool", ""),
		"action":         stringField(inputs, "action", ""),
		"timestamp":      int(numberField(inputs, "timestamp", 0)),
	}
	payloadBytes, err := canonical.Marshal(payload)
	if err != nil {
		return fmt.Errorf("canonicalize MCP payload: %w", err)
	}
	signature := w.Sign(payloadBytes)

	if want, ok := expected["payload_canonical"].(string); ok && string(payloadBytes) != want {
		return fmt.Errorf("payload_canonical mismatch: got %s want %s", string(payloadBytes), want)
	}
	if want, ok := expected["signature_length"].(float64); ok && len(signature) != int(want) {
		return fmt.Errorf("signature length mismatch: got %d want %d", len(signature), int(want))
	}
	if want, ok := expected["frame_type"].(string); ok && want != "DATA" {
		return fmt.Errorf("unsupported MCP frame_type expectation %s", want)
	}

	return nil
}

func checkMCPNegative(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	expected, ok := vec["expected"].(map[string]any)
	if !ok {
		return errors.New("missing expected object")
	}
	if want, ok := expected["verify"].(bool); ok {
		got := stringField(inputs, "actual_correlation_id", "") == stringField(inputs, "expected_correlation_id", "")
		if got != want {
			return fmt.Errorf("verify mismatch: got %t want %t", got, want)
		}
	}
	return nil
}

func encodeFrame(frameType string, payload []byte, version int, flags int) (string, error) {
	frame := map[string]any{
		"version": version,
		"type":    frameType,
		"flags":   flags,
		"payload": base64.RawURLEncoding.EncodeToString(payload),
	}
	canonicalBytes, err := canonical.Marshal(frame)
	if err != nil {
		return "", fmt.Errorf("encode frame: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(canonicalBytes), nil
}

func decodeFrameBase64URL(encoded string) (map[string]any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("frame decoding failed: %w", err)
	}
	return decodeFrameRaw(raw)
}

func decodeFrameRaw(raw []byte) (map[string]any, error) {
	var frame map[string]any
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, fmt.Errorf("frame decoding failed: truncated or invalid JSON: %w", err)
	}

	frameType := stringField(frame, "type", "")
	if frameType == "" {
		return nil, errors.New("frame decoding failed: missing type")
	}
	if !isValidFrameType(frameType) {
		return nil, fmt.Errorf("frame decoding failed: invalid frame type %s", frameType)
	}
	payload := stringField(frame, "payload", "")
	if payload == "" && !hasKey(frame, "payload") {
		return nil, errors.New("frame decoding failed: missing payload")
	}
	if _, err := base64.RawURLEncoding.DecodeString(payload); err != nil {
		return nil, fmt.Errorf("frame decoding failed: invalid payload encoding: %w", err)
	}
	if !hasKey(frame, "version") {
		frame["version"] = float64(1)
	}
	if !hasKey(frame, "flags") {
		frame["flags"] = float64(0)
	}
	return frame, nil
}

func isValidFrameType(frameType string) bool {
	switch frameType {
	case "HANDSHAKE", "HANDSHAKE_ACK", "DATA", "PING", "PONG", "CLOSE":
		return true
	default:
		return false
	}
}

func runHeaderVectors(path string, payload map[string]any) (summary, error) {
	var result summary
	testCases := listField(payload, "test_cases")
	if len(testCases) == 0 {
		return result, fmt.Errorf("%s missing test_cases", path)
	}

	for _, candidate := range testCases {
		vec, ok := candidate.(map[string]any)
		if !ok {
			return result, fmt.Errorf("%s contains malformed test case", path)
		}
		testID := stringField(vec, "id", "unknown")
		if err := checkHeaderVector(vec); err != nil {
			fmt.Printf("FAIL %s: %v\n", testID, err)
			result.failed++
		} else {
			result.passed++
		}
	}

	return result, nil
}

func checkHeaderVector(vec map[string]any) error {
	inputHeader, ok := vec["input_header"].(map[string]any)
	if !ok {
		return errors.New("missing input_header")
	}
	header := map[string]any{
		"dh": stringField(inputHeader, "dh", ""),
		"n":  int(numberField(inputHeader, "n", 0)),
		"pn": int(numberField(inputHeader, "pn", 0)),
	}
	canonicalBytes, err := canonical.Marshal(header)
	if err != nil {
		return fmt.Errorf("canonicalize header: %w", err)
	}
	if want := stringField(vec, "expected_canonical_json", ""); want != "" && string(canonicalBytes) != want {
		return fmt.Errorf("expected canonical json %s, got %s", want, string(canonicalBytes))
	}
	if want := stringField(vec, "expected_canonical_b64u", ""); want != "" {
		got := base64.RawURLEncoding.EncodeToString(canonicalBytes)
		if got != want {
			return fmt.Errorf("expected canonical b64u %s, got %s", want, got)
		}
	}
	return nil
}

func runKdfRootVector(payload map[string]any) (summary, error) {
	testID := stringField(payload, "test_id", "kdf_rk")
	if err := checkKdfRootVector(payload); err != nil {
		fmt.Printf("FAIL %s: %v\n", testID, err)
		return summary{failed: 1}, nil
	}
	return summary{passed: 1}, nil
}

func checkKdfRootVector(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	expected, ok := vec["expected"].(map[string]any)
	if !ok {
		return errors.New("missing expected object")
	}
	rk, err := base64.RawURLEncoding.DecodeString(stringField(inputs, "rk", ""))
	if err != nil {
		return fmt.Errorf("decode rk: %w", err)
	}
	dhOut, err := base64.RawURLEncoding.DecodeString(stringField(inputs, "dh_out", ""))
	if err != nil {
		return fmt.Errorf("decode dh_out: %w", err)
	}
	derived := hkdfSHA256(append(append([]byte{}, rk...), dhOut...), []byte(stringField(inputs, "info", "")), int(numberField(inputs, "out_len", 64)))
	gotRK := base64.RawURLEncoding.EncodeToString(derived[:32])
	gotCK := base64.RawURLEncoding.EncodeToString(derived[32:])
	if want := stringField(expected, "new_rk", ""); want != "" && gotRK != want {
		return fmt.Errorf("new_rk mismatch: got %s want %s", gotRK, want)
	}
	if want := stringField(expected, "new_ck", ""); want != "" && gotCK != want {
		return fmt.Errorf("new_ck mismatch: got %s want %s", gotCK, want)
	}
	return nil
}

func runKdfChainVector(payload map[string]any) (summary, error) {
	testID := stringField(payload, "test_id", "kdf_ck")
	if err := checkKdfChainVector(payload); err != nil {
		fmt.Printf("FAIL %s: %v\n", testID, err)
		return summary{failed: 1}, nil
	}
	return summary{passed: 1}, nil
}

func checkKdfChainVector(vec map[string]any) error {
	inputs, ok := vec["inputs"].(map[string]any)
	if !ok {
		return errors.New("missing inputs object")
	}
	expected, ok := vec["expected"].(map[string]any)
	if !ok {
		return errors.New("missing expected object")
	}
	ck, err := base64.RawURLEncoding.DecodeString(stringField(inputs, "ck", ""))
	if err != nil {
		return fmt.Errorf("decode ck: %w", err)
	}
	length := int(numberField(inputs, "out_len", 32))
	gotMK := base64.RawURLEncoding.EncodeToString(hkdfSHA256(ck, []byte(stringField(inputs, "info_message", "")), length))
	gotNextCK := base64.RawURLEncoding.EncodeToString(hkdfSHA256(ck, []byte(stringField(inputs, "info_chain", "")), length))
	if want := stringField(expected, "mk", ""); want != "" && gotMK != want {
		return fmt.Errorf("mk mismatch: got %s want %s", gotMK, want)
	}
	if want := stringField(expected, "next_ck", ""); want != "" && gotNextCK != want {
		return fmt.Errorf("next_ck mismatch: got %s want %s", gotNextCK, want)
	}
	return nil
}

type ratchetTraceState struct {
	dhPrivate     []byte
	dhPublic      []byte
	dhRemote      []byte
	rootKey       []byte
	chainKeySend  []byte
	chainKeyRecv  []byte
	sendCount     int
	recvCount     int
	prevSendCount int
}

func runRatchetRoundtripVector(payload map[string]any) (summary, error) {
	testID := stringField(payload, "title", "v1_1_0_roundtrip")
	if err := checkRatchetRoundtripVector(payload); err != nil {
		fmt.Printf("FAIL %s: %v\n", testID, err)
		return summary{failed: 1}, nil
	}
	return summary{passed: 1}, nil
}

func checkRatchetRoundtripVector(trace map[string]any) error {
	aliceState, err := initAliceRatchetTraceState(trace)
	if err != nil {
		return err
	}

	var bobState *ratchetTraceState
	steps := listField(trace, "steps")
	if len(steps) == 0 {
		return errors.New("missing ratchet steps")
	}

	for _, candidate := range steps {
		step, ok := candidate.(map[string]any)
		if !ok {
			return errors.New("malformed ratchet step")
		}
		actor := stringField(step, "actor", "")
		action := stringField(step, "action", "")

		switch action {
		case "encrypt":
			var state *ratchetTraceState
			if actor == "alice" {
				state = aliceState
			} else if actor == "bob" {
				state = bobState
			}
			if state == nil {
				return fmt.Errorf("session not initialized for actor %s", actor)
			}
			if err := state.encryptAndCheck(step); err != nil {
				return fmt.Errorf("step %d encrypt: %w", int(numberField(step, "step", 0)), err)
			}
		case "decrypt":
			if actor == "bob" && bobState == nil {
				bobState, err = initBobRatchetTraceState(trace)
				if err != nil {
					return err
				}
			}

			var state *ratchetTraceState
			if actor == "alice" {
				state = aliceState
			} else if actor == "bob" {
				state = bobState
			}
			if state == nil {
				return fmt.Errorf("session not initialized for actor %s", actor)
			}

			if err := state.decryptAndCheck(step); err != nil {
				return fmt.Errorf("step %d decrypt: %w", int(numberField(step, "step", 0)), err)
			}
		default:
			return fmt.Errorf("unsupported ratchet action %s", action)
		}
	}

	return nil
}

func initAliceRatchetTraceState(trace map[string]any) (*ratchetTraceState, error) {
	alice, ok := trace["alice"].(map[string]any)
	if !ok {
		return nil, errors.New("missing alice trace data")
	}
	bob, ok := trace["bob"].(map[string]any)
	if !ok {
		return nil, errors.New("missing bob trace data")
	}
	prekeyBundle, ok := bob["prekey_bundle"].(map[string]any)
	if !ok {
		return nil, errors.New("missing bob prekey bundle")
	}

	alicePrivate, err := base64.RawURLEncoding.DecodeString(stringField(alice, "ephemeral_private", ""))
	if err != nil {
		return nil, fmt.Errorf("decode alice ephemeral private: %w", err)
	}
	alicePublic, err := x25519PublicKey(alicePrivate)
	if err != nil {
		return nil, fmt.Errorf("derive alice ephemeral public: %w", err)
	}
	bobSignedPrekey, err := base64.RawURLEncoding.DecodeString(stringField(prekeyBundle, "signed_prekey", ""))
	if err != nil {
		return nil, fmt.Errorf("decode bob signed_prekey: %w", err)
	}

	dhOut, err := x25519DH(alicePrivate, bobSignedPrekey)
	if err != nil {
		return nil, fmt.Errorf("alice initial dh: %w", err)
	}
	rootKey := hkdfSHA256(dhOut, []byte("x3dh-init"), 32)
	rootKey, chainKeySend := ratchetKdfRoot(rootKey, dhOut)

	return &ratchetTraceState{
		dhPrivate:     alicePrivate,
		dhPublic:      alicePublic,
		dhRemote:      bobSignedPrekey,
		rootKey:       rootKey,
		chainKeySend:  chainKeySend,
		chainKeyRecv:  nil,
		sendCount:     0,
		recvCount:     0,
		prevSendCount: 0,
	}, nil
}

func initBobRatchetTraceState(trace map[string]any) (*ratchetTraceState, error) {
	alice, ok := trace["alice"].(map[string]any)
	if !ok {
		return nil, errors.New("missing alice trace data")
	}
	bob, ok := trace["bob"].(map[string]any)
	if !ok {
		return nil, errors.New("missing bob trace data")
	}
	prekeyBundle, ok := bob["prekey_bundle"].(map[string]any)
	if !ok {
		return nil, errors.New("missing bob prekey bundle")
	}
	bundleSecrets, ok := bob["bundle_secrets"].(map[string]any)
	if !ok {
		return nil, errors.New("missing bob bundle secrets")
	}

	alicePrivate, err := base64.RawURLEncoding.DecodeString(stringField(alice, "ephemeral_private", ""))
	if err != nil {
		return nil, fmt.Errorf("decode alice ephemeral private: %w", err)
	}
	alicePublic, err := x25519PublicKey(alicePrivate)
	if err != nil {
		return nil, fmt.Errorf("derive alice ephemeral public: %w", err)
	}
	bobSignedPrekeyPrivate, err := base64.RawURLEncoding.DecodeString(stringField(bundleSecrets, "signed_prekey_private", ""))
	if err != nil {
		return nil, fmt.Errorf("decode bob signed_prekey_private: %w", err)
	}
	bobSignedPrekeyPublic, err := base64.RawURLEncoding.DecodeString(stringField(prekeyBundle, "signed_prekey", ""))
	if err != nil {
		return nil, fmt.Errorf("decode bob signed_prekey: %w", err)
	}

	dhOut, err := x25519DH(bobSignedPrekeyPrivate, alicePublic)
	if err != nil {
		return nil, fmt.Errorf("bob initial dh: %w", err)
	}
	rootKey := hkdfSHA256(dhOut, []byte("x3dh-init"), 32)
	rootKey, chainKeyRecv := ratchetKdfRoot(rootKey, dhOut)

	return &ratchetTraceState{
		dhPrivate:     bobSignedPrekeyPrivate,
		dhPublic:      bobSignedPrekeyPublic,
		dhRemote:      alicePublic,
		rootKey:       rootKey,
		chainKeySend:  nil,
		chainKeyRecv:  chainKeyRecv,
		sendCount:     0,
		recvCount:     0,
		prevSendCount: 0,
	}, nil
}

func (s *ratchetTraceState) encryptAndCheck(step map[string]any) error {
	if len(s.chainKeySend) == 0 {
		ratchetPrivate, err := base64.RawURLEncoding.DecodeString(stringField(step, "ratchet_priv", ""))
		if err != nil {
			return fmt.Errorf("decode ratchet_priv: %w", err)
		}
		if err := s.initializeSendingChain(ratchetPrivate); err != nil {
			return err
		}
	}

	plaintext, err := base64.RawURLEncoding.DecodeString(stringField(step, "plaintext", ""))
	if err != nil {
		return fmt.Errorf("decode plaintext: %w", err)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(stringField(step, "nonce", ""))
	if err != nil {
		return fmt.Errorf("decode nonce: %w", err)
	}

	header := map[string]any{
		"dh": base64.RawURLEncoding.EncodeToString(s.dhPublic),
		"n":  s.sendCount,
		"pn": s.prevSendCount,
	}
	headerBytes, err := canonical.Marshal(header)
	if err != nil {
		return fmt.Errorf("canonicalize header: %w", err)
	}

	messageKey, nextChainKey := ratchetKdfChain(s.chainKeySend)
	ciphertext, err := encryptChaCha20Poly1305(messageKey, nonce, plaintext, headerBytes)
	if err != nil {
		return err
	}
	s.chainKeySend = nextChainKey
	s.sendCount++

	ciphertextB64 := base64.RawURLEncoding.EncodeToString(ciphertext)
	if want := stringField(step, "ciphertext", ""); want != "" && ciphertextB64 != want {
		return fmt.Errorf("ciphertext mismatch: got %s want %s", ciphertextB64, want)
	}
	if want := stringField(step, "aad", ""); want != "" {
		gotAAD := base64.RawURLEncoding.EncodeToString(headerBytes)
		if gotAAD != want {
			return fmt.Errorf("aad mismatch: got %s want %s", gotAAD, want)
		}
	}

	envelope := map[string]any{
		"header":     header,
		"nonce":      base64.RawURLEncoding.EncodeToString(nonce),
		"ciphertext": ciphertextB64,
	}
	wireBytes, err := canonical.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("canonicalize envelope: %w", err)
	}
	wireB64 := base64.RawURLEncoding.EncodeToString(wireBytes)
	if want := stringField(step, "wire_message_b64u", ""); want != "" && wireB64 != want {
		return fmt.Errorf("wire_message mismatch: got %s want %s", wireB64, want)
	}

	return nil
}

func (s *ratchetTraceState) decryptAndCheck(step map[string]any) error {
	wireBytes, err := base64.RawURLEncoding.DecodeString(stringField(step, "wire_message_b64u", ""))
	if err != nil {
		return fmt.Errorf("decode wire_message_b64u: %w", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(wireBytes, &envelope); err != nil {
		return fmt.Errorf("decode ratchet envelope: %w", err)
	}
	header, ok := envelope["header"].(map[string]any)
	if !ok {
		return errors.New("missing envelope header")
	}
	headerBytes, err := canonical.Marshal(map[string]any{
		"dh": stringField(header, "dh", ""),
		"n":  int(numberField(header, "n", 0)),
		"pn": int(numberField(header, "pn", 0)),
	})
	if err != nil {
		return fmt.Errorf("canonicalize header: %w", err)
	}

	headerDH, err := base64.RawURLEncoding.DecodeString(stringField(header, "dh", ""))
	if err != nil {
		return fmt.Errorf("decode header dh: %w", err)
	}
	if !equalBytes(headerDH, s.dhRemote) {
		if err := s.skipMessageKeys(int(numberField(header, "pn", 0))); err != nil {
			return err
		}
		if err := s.dhRatchet(headerDH); err != nil {
			return err
		}
	}

	if err := s.skipMessageKeys(int(numberField(header, "n", 0))); err != nil {
		return err
	}
	if len(s.chainKeyRecv) == 0 {
		return errors.New("missing receiving chain key")
	}

	nonce, err := base64.RawURLEncoding.DecodeString(stringField(envelope, "nonce", ""))
	if err != nil {
		return fmt.Errorf("decode envelope nonce: %w", err)
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(stringField(envelope, "ciphertext", ""))
	if err != nil {
		return fmt.Errorf("decode envelope ciphertext: %w", err)
	}

	messageKey, nextChainKey := ratchetKdfChain(s.chainKeyRecv)
	plaintext, err := decryptChaCha20Poly1305(messageKey, nonce, ciphertext, headerBytes)
	if err != nil {
		return fmt.Errorf("decrypt ratchet message: %w", err)
	}
	s.chainKeyRecv = nextChainKey
	s.recvCount++

	expectedPlaintext, err := base64.RawURLEncoding.DecodeString(stringField(step, "expected_plaintext", ""))
	if err != nil {
		return fmt.Errorf("decode expected_plaintext: %w", err)
	}
	if !equalBytes(plaintext, expectedPlaintext) {
		return fmt.Errorf(
			"plaintext mismatch: got %s want %s",
			base64.RawURLEncoding.EncodeToString(plaintext),
			base64.RawURLEncoding.EncodeToString(expectedPlaintext),
		)
	}

	return nil
}

func (s *ratchetTraceState) initializeSendingChain(privateKey []byte) error {
	if len(privateKey) != 32 {
		return errors.New("ratchet private key must be 32 bytes")
	}
	publicKey, err := x25519PublicKey(privateKey)
	if err != nil {
		return fmt.Errorf("derive ratchet public key: %w", err)
	}

	s.prevSendCount = s.sendCount
	s.sendCount = 0
	s.dhPrivate = append([]byte{}, privateKey...)
	s.dhPublic = publicKey

	dhOut, err := x25519DH(s.dhPrivate, s.dhRemote)
	if err != nil {
		return fmt.Errorf("initialize sending chain dh: %w", err)
	}
	s.rootKey, s.chainKeySend = ratchetKdfRoot(s.rootKey, dhOut)
	return nil
}

func (s *ratchetTraceState) skipMessageKeys(until int) error {
	if len(s.chainKeyRecv) == 0 {
		return nil
	}
	if s.recvCount+1000 < until {
		return errors.New("too many skipped ratchet messages")
	}

	for s.recvCount < until {
		_, nextChainKey := ratchetKdfChain(s.chainKeyRecv)
		s.chainKeyRecv = nextChainKey
		s.recvCount++
	}
	return nil
}

func (s *ratchetTraceState) dhRatchet(remotePublic []byte) error {
	s.prevSendCount = s.sendCount
	s.sendCount = 0
	s.recvCount = 0
	s.dhRemote = append([]byte{}, remotePublic...)

	dhRecv, err := x25519DH(s.dhPrivate, s.dhRemote)
	if err != nil {
		return fmt.Errorf("dh ratchet receive: %w", err)
	}
	s.rootKey, s.chainKeyRecv = ratchetKdfRoot(s.rootKey, dhRecv)

	nextPrivate, nextPublic, err := generateX25519Keypair()
	if err != nil {
		return fmt.Errorf("generate next ratchet key: %w", err)
	}
	s.dhPrivate = nextPrivate
	s.dhPublic = nextPublic

	dhSend, err := x25519DH(s.dhPrivate, s.dhRemote)
	if err != nil {
		return fmt.Errorf("dh ratchet send: %w", err)
	}
	s.rootKey, s.chainKeySend = ratchetKdfRoot(s.rootKey, dhSend)
	return nil
}

func ratchetKdfRoot(rootKey, dhOut []byte) ([]byte, []byte) {
	derived := hkdfSHA256(append(append([]byte{}, rootKey...), dhOut...), []byte("talos-double-ratchet-root"), 64)
	return append([]byte{}, derived[:32]...), append([]byte{}, derived[32:]...)
}

func ratchetKdfChain(chainKey []byte) ([]byte, []byte) {
	messageKey := hkdfSHA256(chainKey, []byte("talos-double-ratchet-message"), 32)
	nextChainKey := hkdfSHA256(chainKey, []byte("talos-double-ratchet-chain"), 32)
	return messageKey, nextChainKey
}

func x25519PublicKey(privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, errors.New("x25519 private key must be 32 bytes")
	}
	curve := ecdh.X25519()
	key, err := curve.NewPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	return append([]byte{}, key.PublicKey().Bytes()...), nil
}

func x25519DH(privateKey, publicKey []byte) ([]byte, error) {
	if len(privateKey) != 32 || len(publicKey) != 32 {
		return nil, errors.New("x25519 keys must be 32 bytes")
	}
	curve := ecdh.X25519()
	private, err := curve.NewPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	public, err := curve.NewPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	sharedSecret, err := private.ECDH(public)
	if err != nil {
		return nil, err
	}
	return append([]byte{}, sharedSecret...), nil
}

func generateX25519Keypair() ([]byte, []byte, error) {
	curve := ecdh.X25519()
	private, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return append([]byte{}, private.Bytes()...), append([]byte{}, private.PublicKey().Bytes()...), nil
}

func encryptChaCha20Poly1305(key, nonce, plaintext, ad []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid chacha20poly1305 key size %d", len(key))
	}
	if len(nonce) != chacha20poly1305.NonceSize {
		return nil, fmt.Errorf("invalid chacha20poly1305 nonce size %d", len(nonce))
	}
	cipher, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return cipher.Seal(nil, nonce, plaintext, ad), nil
}

func decryptChaCha20Poly1305(key, nonce, ciphertext, ad []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid chacha20poly1305 key size %d", len(key))
	}
	if len(nonce) != chacha20poly1305.NonceSize {
		return nil, fmt.Errorf("invalid chacha20poly1305 nonce size %d", len(nonce))
	}
	cipher, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return cipher.Open(nil, nonce, ciphertext, ad)
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func listField(payload map[string]any, key string) []any {
	values, ok := payload[key].([]any)
	if !ok {
		return nil
	}
	return values
}

func stringField(payload map[string]any, key, fallback string) string {
	value, ok := payload[key].(string)
	if !ok {
		return fallback
	}
	return value
}

func hasKey(payload map[string]any, key string) bool {
	_, ok := payload[key]
	return ok
}

func numberField(payload map[string]any, key string, fallback float64) float64 {
	value, ok := payload[key].(float64)
	if !ok {
		return fallback
	}
	return value
}

const (
	seedHexOne                = "0000000000000000000000000000000000000000000000000000000000000001"
	seedHexTwo                = "0000000000000000000000000000000000000000000000000000000000000002"
	defaultCapabilityIssuedAt = 1704067200
	defaultCapabilityExpiry   = 1767504000
	defaultCapabilitySubject  = "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
)

func defaultCapabilityScope() []map[string]any {
	return []map[string]any{
		{
			"tool":    "filesystem",
			"actions": []string{"read", "write"},
		},
	}
}

func makeSignedCapability(inputs map[string]any) (map[string]any, []byte, int, error) {
	seedBytes, err := hex.DecodeString(stringField(inputs, "issuer_seed_hex", seedHexOne))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("decode issuer_seed_hex: %w", err)
	}
	w, err := wallet.FromSeed(seedBytes, "")
	if err != nil {
		return nil, nil, 0, fmt.Errorf("wallet from issuer seed: %w", err)
	}
	exp := int(numberField(inputs, "exp", float64(defaultCapabilityExpiry)))
	capability := map[string]any{
		"v":     "1",
		"iss":   w.DID(),
		"sub":   stringField(inputs, "subject_did", defaultCapabilitySubject),
		"scope": inputs["scope"],
		"iat":   numberField(inputs, "iat", float64(defaultCapabilityIssuedAt)),
		"exp":   float64(exp),
	}
	canonicalBytes, err := canonical.Marshal(capability)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("canonicalize capability: %w", err)
	}
	capability["sig"] = base64.RawURLEncoding.EncodeToString(w.Sign(canonicalBytes))
	return capability, w.PublicKey(), exp, nil
}

func verifyCapability(capability map[string]any, issuerPublicKey []byte, now int) (bool, string, error) {
	signatureEncoded := stringField(capability, "sig", "")
	if signatureEncoded == "" {
		return false, "signature", nil
	}
	if int(numberField(capability, "exp", 0)) < now {
		return false, "expired", nil
	}
	content := make(map[string]any, len(capability)-1)
	for key, value := range capability {
		if key == "sig" {
			continue
		}
		content[key] = value
	}
	canonicalBytes, err := canonical.Marshal(content)
	if err != nil {
		return false, "", fmt.Errorf("canonicalize capability content: %w", err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(signatureEncoded)
	if err != nil {
		return false, "signature", nil
	}
	if !wallet.Verify(issuerPublicKey, canonicalBytes, signature) {
		return false, "signature", nil
	}
	return true, "", nil
}

func capabilityAuthorize(scopeValue any, tool, action string) bool {
	scopes, ok := scopeValue.([]any)
	if !ok {
		if typed, ok := scopeValue.([]map[string]any); ok {
			for _, scope := range typed {
				if capabilityScopeMatches(scope, tool, action) {
					return true
				}
			}
		}
		return false
	}
	for _, candidate := range scopes {
		scope, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if capabilityScopeMatches(scope, tool, action) {
			return true
		}
	}
	return false
}

func capabilityScopeMatches(scope map[string]any, tool, action string) bool {
	if stringField(scope, "tool", "") != tool {
		return false
	}
	actions := listField(scope, "actions")
	for _, candidate := range actions {
		value, ok := candidate.(string)
		if ok && value == action {
			return true
		}
	}
	return false
}

func splitAuthorizeKey(key string) []string {
	const prefix = "authorize_fast_"
	raw := key[len(prefix):]
	for idx := 0; idx < len(raw); idx++ {
		if raw[idx] != '_' {
			continue
		}
		return []string{raw[:idx], raw[idx+1:]}
	}
	return nil
}

func hkdfSHA256(ikm, info []byte, length int) []byte {
	salt := make([]byte, sha256.Size)
	prk := hmacSHA256(salt, ikm)
	output := make([]byte, 0, length)
	var block []byte
	for counter := byte(1); len(output) < length; counter++ {
		mac := hmac.New(sha256.New, prk)
		if len(block) > 0 {
			mac.Write(block)
		}
		mac.Write(info)
		mac.Write([]byte{counter})
		block = mac.Sum(nil)
		output = append(output, block...)
	}
	return output[:length]
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
