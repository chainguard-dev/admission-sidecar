//go:build e2e
// +build e2e

/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package e2e

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	v1 "k8s.io/api/admission/v1"
)

var (
	hookToTest   = flag.String("hook", "policy.sigstore.dev", "Which webhook to target through the proxy")
	proxyURL     = flag.String("url", "http://localhost:8088", "URL of the proxy")
	requireLabel = flag.Bool("require-label", false, "Is proxy configured to require NS to be labeled.")
	// testnamespaces holds the namespaces that we want to test. For each
	// namespace, the bool indicates
	testnamespaces = map[string]bool{
		"test-labeled-include":        true,
		"test-labeled-do-not-include": false,
		"test-not-labeled":            false}
)

// placeholder is used as a placeholder in the admission request to swap
// in the appropriate namespace as it pertains to the test.
const placeholder = "TEST_NAMESPACE_REPLACE_ME"

func TestMain(m *testing.M) {
	flag.Parse()

	if *hookToTest == "" {
		log.Fatalf("Must give hook to test against (--hook)")
	}
	os.Exit(m.Run())
}

func mutateURL() string {
	return fmt.Sprintf("%s/mutate/%s", strings.TrimRight(*proxyURL, "/"), *hookToTest)
}

func admitURL() string {
	return fmt.Sprintf("%s/admit/%s", strings.TrimRight(*proxyURL, "/"), *hookToTest)
}

// setNamespace does basically a sed replacing the sentinel namespace
// with the one for the appropriate test.
func setNamespace(in []byte, ns string) []byte {
	return []byte(strings.ReplaceAll(string(in), placeholder, ns))
}

// TestE2EAdmit tests admission for the following test cases:
// * image not fully specified, auto-reject
// * image fully specified, but static fail policy against it
// * image fully specified, pass
// treats all the namespaces the same.
func TestE2EAdmit(t *testing.T) {
	tests := []struct {
		name       string
		testfile   string
		allowed    bool
		wantErrMsg string
	}{{
		name:       "Full image, want static fail",
		testfile:   "testrequest-full-image.json",
		wantErrMsg: "disallowed by static policy",
	}, {
		name:       "Not full image, fail",
		testfile:   "testrequest.json",
		wantErrMsg: "nginx must be an image digest: spec.containers[0].image",
	}, {
		name:     "Full image, want static pass",
		testfile: "testrequest-full-image-allowed.json",
		allowed:  true,
	}}
	for _, tc := range tests {
		body, err := ioutil.ReadFile("../testdata/" + tc.testfile)
		if err != nil {
			t.Fatalf("Failed to read file %s: %s", "file", err)
		}
		for ns, isEnforced := range testnamespaces {
			// Fix the namespace on the admission request
			nsBody := setNamespace(body, ns)
			got, err := doRequest(admitURL(), nsBody)
			if err != nil {
				t.Errorf("Failed the doRequest: %s", err)
			}
			// If namespace is not enforced, and the proxy is operating in
			// require-label mode, it must always be pass
			if !isEnforced && *requireLabel == true && got.Allowed != true {
				t.Errorf("%q Blocked when ns is not enforced for %s", tc.name, ns)
				continue
			}
			if !isEnforced && *requireLabel == true && got.Allowed == true {
				// Not being enforced and requirelabel is true and it passes
				// checks out.
				continue
			}
			if got.Allowed != tc.allowed {
				t.Errorf("%q Allowed mismatch for %s want %v got %v", tc.name, ns, tc.allowed, got.Allowed)
			}
			if !tc.allowed && tc.wantErrMsg != "" {
				// Check the error message
				if got.Result == nil {
					t.Errorf("%q Wanted error msg in %s: %q but got none", tc.name, ns, tc.wantErrMsg)
				} else {
					if !strings.Contains(got.Result.Message, tc.wantErrMsg) {
						t.Errorf("%q Wanted error msg in %s: %q got: %s", tc.name, ns, tc.wantErrMsg, got.Result.Message)
					}
				}
			}
		}
	}
}

func TestE2EMutate(t *testing.T) {
	tests := []struct {
		name      string
		testfile  string
		allowed   bool
		wantPatch string
	}{{
		name:     "Full image, want no patch",
		testfile: "testrequest-full-image.json",
		allowed:  true,
	}, {
		name:      "Not full image, want patch",
		testfile:  "testrequest.json",
		allowed:   true,
		wantPatch: `"path":"/spec/containers/0/image`,
	}}
	//body, err := ioutil.ReadFile("../testdata/testrequest-full-image.json")
	for _, tc := range tests {
		body, err := ioutil.ReadFile("../testdata/" + tc.testfile)
		if err != nil {
			t.Fatalf("Failed to read file %s: %s", "file", err)
		}
		for ns, isEnforced := range testnamespaces {
			// Fix the namespace on the admission request
			nsbody := setNamespace(body, ns)
			got, err := doRequest(mutateURL(), nsbody)
			if err != nil {
				t.Errorf("Failed the doRequest: %s", err)
			}
			// If namespace is not enforced, and the proxy is operating in
			// require-label mode, it must not patch
			if !isEnforced && *requireLabel == true && (got.Patch != nil && bytes.Compare(got.Patch, []byte("null")) != 0) {
				t.Errorf("Patched when ns is not enforced")
				continue
			}
			if !isEnforced && *requireLabel == true && (got.Patch == nil || bytes.Compare(got.Patch, []byte("null")) == 0) {
				// Not being enforced and requirelabel is true and there's no
				// patch, so checks out.
				continue
			}
			if got.Allowed != tc.allowed {
				t.Errorf("%q %q Allowed mismatch want %v got %v", tc.name, ns, tc.allowed, got.Allowed)
			}
			// Check the error message
			if tc.wantPatch != "" && got.Patch == nil {
				t.Errorf("%q %q Wanted patch %q got none", tc.name, ns, tc.wantPatch)
			} else if tc.wantPatch == "" && got.Patch != nil {
				// It could be a null patch so check for that.
				if bytes.Compare(got.Patch, []byte("null")) != 0 {
					t.Errorf("%q %q Did not watch patch got %+v", tc.name, ns, got.Patch)
				}
			} else if !strings.Contains(string(got.Patch), tc.wantPatch) {
				t.Errorf("%q %q Wanted patch %q got %s", tc.name, ns, tc.wantPatch, got.Patch)
			}
		}
	}
}

func doRequest(url string, body []byte) (*v1.AdmissionResponse, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ret := &v1.AdmissionReview{}
	err = json.Unmarshal(response, ret)
	if err != nil {
		return nil, err
	}
	return ret.Response, nil
}
