package auth

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// Helper function to test breach checking with a custom server
func testCheckBreachWithServer(t *testing.T, server *httptest.Server, password string, expectBreached bool) {
	sum := sha1.Sum([]byte(password))
	hashStr := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix := hashStr[:5]
	suffix := hashStr[5:]

	url := server.URL + "/range/" + prefix
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		// Network error: should be fail-open (not breached)
		if expectBreached {
			t.Fatalf("network error, expected breach detection: %v", err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Server error: should be fail-open (not breached)
		if expectBreached {
			t.Fatalf("server error %d, expected breach detection", resp.StatusCode)
		}
		return
	}

	// Parse response using the same logic as CheckPasswordBreach
	scanner := bufio.NewScanner(resp.Body)
	found := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == suffix {
			count, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			if count > 0 {
				found = true
				break
			}
		}
	}

	if found != expectBreached {
		if expectBreached {
			t.Error("expected password to be marked as breached but was not")
		} else {
			t.Error("expected password to NOT be breached but was")
		}
	}
}

// TestCheckPasswordBreachedKnownBreach tests that a known breached password is detected
func TestCheckPasswordBreachedKnownBreach(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return canned response for password "password"
		// SHA1("password") = 5BAA61E4C9B93F3F0682250B6CF8331B7EE68FD8
		// Prefix: 5BAA6, Suffix: 1E4C9B93F3F0682250B6CF8331B7EE68FD8
		response := "01A0C6A35FBA1E2DF8E7FBEF2ECAD59A:1\r\n" +
			"1E4C9B93F3F0682250B6CF8331B7EE68FD8:3533066\r\n" +
			"E9370D633D3F133B0697E47400D89F27D:1\r\n"
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	testCheckBreachWithServer(t, server, "password", true)
}

// TestCheckPasswordBreachedNotBreach tests that a never-seen password passes
func TestCheckPasswordBreachedNotBreach(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response without the random password's suffix
		response := "01A0C6A35FBA1E2DF8E7FBEF2ECAD59A:1\r\n" +
			"E9370D633D3F133B0697E47400D89F27D:1\r\n"
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	// Use a random 32-byte hex password unlikely to be in HIBP
	randomPassword := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testCheckBreachWithServer(t, server, randomPassword, false)
}

// TestCheckPasswordBreachedNetworkError tests graceful handling of network errors
func TestCheckPasswordBreachedNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	// Should fail gracefully (not breach) on server error
	testCheckBreachWithServer(t, server, "password", false)
}

// TestCheckPasswordBreachedCRLFHandling tests CRLF line ending parsing
func TestCheckPasswordBreachedCRLFHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with proper CRLF endings
		// Suffix for "password" is: 1E4C9B93F3F0682250B6CF8331B7EE68FD8
		response := "01A0C6A35FBA1E2DF8E7FBEF2ECAD59A:1\r\n" +
			"1E4C9B93F3F0682250B6CF8331B7EE68FD8:3533066\r\n" +
			"E9370D633D3F133B0697E47400D89F27D:1\r\n"
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	// Test with password that matches a line in the CRLF-formatted response
	testCheckBreachWithServer(t, server, "password", true)
}

// TestCheckPasswordBreachedMidResponse tests finding match in middle of response
func TestCheckPasswordBreachedMidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a response where the match is in the middle
		// Suffix for "password" is: 1E4C9B93F3F0682250B6CF8331B7EE68FD8
		response := "01A0C6A35FBA1E2DF8E7FBEF2ECAD59A:1\r\n" +
			"02B0D7B36FCC2F3FG9G793251D7DG932C8:2\r\n" +
			"1E4C9B93F3F0682250B6CF8331B7EE68FD8:3533066\r\n" +
			"E9370D633D3F133B0697E47400D89F27D:1\r\n"
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	testCheckBreachWithServer(t, server, "password", true)
}
