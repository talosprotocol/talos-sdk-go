package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/talosprotocol/talos-sdk-go/pkg/talos/errors"
	"github.com/talosprotocol/talos-sdk-go/pkg/talos/wallet"
)

type VectorFile struct {
	Vectors       []TestVector `json:"vectors"`
	NegativeCases []TestVector `json:"negative_cases"`
}

type TestVector struct {
	TestID        string                 `json:"test_id"`
	Inputs        map[string]interface{} `json:"inputs"`
	Expected      map[string]interface{} `json:"expected"`
	ExpectedError *ExpectedError         `json:"expected_error"`
}

type ExpectedError struct {
	Code            string `json:"code"`
	MessageContains string `json:"message_contains"`
}

// JUnit structs
type TestSuites struct {
	XMLName string      `xml:"testsuites"`
	Suites  []TestSuite `xml:"testsuite"`
}

type TestSuite struct {
	XMLName   string     `xml:"testsuite"`
	Name      string     `xml:"name,attr"`
	Tests     int        `xml:"tests,attr"`
	Failures  int        `xml:"failures,attr"`
	Errors    int        `xml:"errors,attr"`
	Time      string     `xml:"time,attr"`
	TestCases []TestCase `xml:"testcase"`
}

type TestCase struct {
	XMLName   string   `xml:"testcase"`
	Name      string   `xml:"name,attr"`
	ClassName string   `xml:"classname,attr"`
	Time      string   `xml:"time,attr"`
	Failure   *Failure `xml:"failure,omitempty"`
	Error     *Error   `xml:"error,omitempty"`
}

type Failure struct {
	Message string `xml:"message,attr"`
	Content string `xml:",chardata"`
}

type Error struct {
	Message string `xml:"message,attr"`
	Content string `xml:",chardata"`
}

func main() {
	vectorsPath := flag.String("vectors", "", "Path to test vector JSON file")
	reportPath := flag.String("report", "", "Path to write JUnit XML report")
	flag.Parse()

	if *vectorsPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --vectors argument is required")
		os.Exit(1)
	}

	data, err := ioutil.ReadFile(*vectorsPath) // deprecated but std
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(2)
	}

	var vf VectorFile
	if err := json.Unmarshal(data, &vf); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(2)
	}

	suiteName := "Conformance" // could derive from file
	suite := TestSuite{Name: suiteName}

	total := 0
	failures := 0
	errs := 0
	start := time.Now()

	// Run positive
	for _, vec := range vf.Vectors {
		total++
		t0 := time.Now()
		err := runVector(vec, false)
		duration := time.Since(t0).Seconds()

		tc := TestCase{Name: vec.TestID, ClassName: suiteName, Time: fmt.Sprintf("%.4f", duration)}
		if err != nil {
			failures++
			fmt.Printf("[FAIL] %s: %v\n", vec.TestID, err)
			tc.Failure = &Failure{Message: err.Error(), Content: err.Error()}
		}
		suite.TestCases = append(suite.TestCases, tc)
	}

	// Run negative
	for _, vec := range vf.NegativeCases {
		total++
		t0 := time.Now()
		err := runVector(vec, true)
		duration := time.Since(t0).Seconds()

		tc := TestCase{Name: vec.TestID, ClassName: suiteName, Time: fmt.Sprintf("%.4f", duration)}
		if err != nil {
			failures++
			fmt.Printf("[FAIL] %s: %v\n", vec.TestID, err)
			tc.Failure = &Failure{Message: err.Error(), Content: err.Error()}
		}
		suite.TestCases = append(suite.TestCases, tc)
	}

	suite.Tests = total
	suite.Failures = failures
	suite.Errors = errs
	suite.Time = fmt.Sprintf("%.4f", time.Since(start).Seconds())

	// Generate report
	if *reportPath != "" {
		suites := TestSuites{Suites: []TestSuite{suite}}
		bytes, _ := xml.MarshalIndent(suites, "", "  ")
		_ = ioutil.WriteFile(*reportPath, append([]byte(xml.Header), bytes...), 0644)
		fmt.Printf("Report written to %s\n", *reportPath)
	}

	fmt.Printf("Ran %d tests in %s\n", total, suite.Time)
	if failures > 0 || errs > 0 {
		fmt.Printf("FAILED (failures=%d)\n", failures)
		os.Exit(1)
	} else {
		fmt.Println("OK")
	}
}

func runVector(vec TestVector, isNegative bool) error {
	var err error
	if strings.HasPrefix(vec.TestID, "sign_") || strings.HasPrefix(vec.TestID, "invalid_seed") {
		err = testSign(vec)
	} else if strings.HasPrefix(vec.TestID, "verify_") {
		err = testVerify(vec)
	} else {
		// skip unknown or implement generic
		return nil
	}

	if isNegative {
		if err == nil {
			// Expected error but got none
			// Unless verified: false was expected
			if expectedVerify, ok := vec.Expected["verify"]; ok {
				if val, ok := expectedVerify.(bool); ok && !val {
					// Expected verify=false, and logic managed it, passed.
					return nil
				}
			}
			return fmt.Errorf("expected error but operation succeeded")
		}

		// If we got an error, check if it matches expected
		if vec.ExpectedError != nil {
			// Check code/message
			// Simplified check
			if vec.ExpectedError.MessageContains != "" {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(vec.ExpectedError.MessageContains)) {
					return fmt.Errorf("error message mismatch: want '%s', got '%s'", vec.ExpectedError.MessageContains, err.Error())
				}
			}
			// Code check requires casting to TalosError
			if vec.ExpectedError.Code != "" {
				if te, ok := err.(*errors.TalosError); ok {
					if string(te.Code) != vec.ExpectedError.Code {
						return fmt.Errorf("error code mismatch: want %s, got %s", vec.ExpectedError.Code, te.Code)
					}
				}
			}
			return nil // Matched expected error
		}

		// Got error but no expected_error spec?
		return nil
	} else {
		// Positive case
		return err
	}
}

func testSign(vec TestVector) error {
	inputs := vec.Inputs
	expected := vec.Expected

	seedHex, _ := inputs["seed_hex"].(string)
	msgStr, _ := inputs["message_utf8"].(string)

	var w *wallet.Wallet

	if seedHex != "" {
		seedBytes, err := hex.DecodeString(seedHex)
		if err != nil {
			return err
		}
		w, err = wallet.FromSeed(seedBytes, "")
		if err != nil {
			return err
		}
	} else {
		return nil // skip
	}

	if did, ok := expected["did"].(string); ok {
		if w.DID() != did {
			return fmt.Errorf("DID mismatch: want %s, got %s", did, w.DID())
		}
	}

	sig := w.Sign([]byte(msgStr))

	if expectedSigB64, ok := expected["signature_base64url"].(string); ok {
		// Go base64 URL encoding might lack padding
		encoded := base64.RawURLEncoding.EncodeToString(sig)
		if encoded != expectedSigB64 {
			return fmt.Errorf("signature mismatch: want %s, got %s", expectedSigB64, encoded)
		}
	}

	if expectedLen, ok := expected["signature_length"].(float64); ok {
		if len(sig) != int(expectedLen) {
			return fmt.Errorf("len mismatch")
		}
	}

	if verify, ok := expected["verify"].(bool); ok && verify {
		if !wallet.Verify(w.PublicKey(), []byte(msgStr), sig) {
			return fmt.Errorf("self verification failed")
		}
	}

	return nil
}

func testVerify(vec TestVector) error {
	inputs := vec.Inputs
	expected := vec.Expected

	msgStr, _ := inputs["message_utf8"].(string)
	// tampered
	if val, ok := inputs["tampered_message"].(string); ok {
		msgStr = val
	}

	var pubKeyBytes []byte
	if val, ok := inputs["public_key_hex"].(string); ok {
		pubKeyBytes, _ = hex.DecodeString(val)
	} else if val, ok := inputs["wrong_public_key_hex"].(string); ok {
		pubKeyBytes, _ = hex.DecodeString(val)
	} else if val, ok := inputs["seed_hex"].(string); ok {
		s, _ := hex.DecodeString(val)
		w, _ := wallet.FromSeed(s, "")
		pubKeyBytes = w.PublicKey()
	}

	var sigBytes []byte
	if val, ok := inputs["signature_base64url"].(string); ok {
		// Use RawURLEncoding for base64url without padding
		b, err := base64.RawURLEncoding.DecodeString(val)
		if err != nil {
			// Try standard if raw fails, strictly base64url usually no padding
			return err
		}
		sigBytes = b
	}

	success := wallet.Verify(pubKeyBytes, []byte(msgStr), sigBytes)

	if val, ok := expected["verify"].(bool); ok {
		if success != val {
			return fmt.Errorf("verification result mismatch: want %v, got %v", val, success)
		}
	}

	return nil
}
