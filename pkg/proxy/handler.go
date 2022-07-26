/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typesv1 "k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
)

// Delegate normalizes the URL/Service endpoint as well as any necessary
// CACerts.
type Delegate struct {
	Service    string
	CACertPool *x509.CertPool
}

func CreateFailResponse(uid typesv1.UID, msg string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		UID:     uid,
		Allowed: false,
		Result: &metav1.Status{
			Code:    http.StatusInternalServerError,
			Message: msg,
		},
	}
}

// WebhookClientConfigToURL normalizes WebhookClientConfig into URL in a string
// representation.
func WebhookClientConfigToURLAndCert(wcc v1.WebhookClientConfig) (*Delegate, error) {
	ret := &Delegate{}
	var caCertPool *x509.CertPool
	if len(wcc.CABundle) > 0 {
		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(wcc.CABundle) {
			return nil, fmt.Errorf("Failed to parse certs from CABundle")
		}
		ret.CACertPool = caCertPool
	}

	if wcc.URL != nil {
		ret.Service = *wcc.URL
		return ret, nil
	}
	// Normalize the Service to string
	svc := wcc.Service
	var port int32 = 443
	if svc.Port != nil {
		port = *svc.Port
	}
	path := "/"
	if svc.Path != nil {
		path = *svc.Path
	}
	ret.Service = fmt.Sprintf("https://%s.%s.svc:%d%s", svc.Name, svc.Namespace, port, path)
	return ret, nil
}

// GetHookName takes in an HTTP request and parses out the targeted webhook
// or an Error if it can't be found.
func GetHookName(ctx context.Context, prefix string, uid typesv1.UID, req *http.Request) (string, *admissionv1.AdmissionResponse) {
	if req == nil {
		logging.FromContext(ctx).Errorf("The context was not properly setup")
		return "", CreateFailResponse(uid, "The context was not properly setup")
	}
	if !strings.HasPrefix(req.URL.Path, prefix) {
		logging.FromContext(ctx).Errorf("path prefix not found for %s in %s", prefix, req.URL.Path)
		return "", CreateFailResponse(uid, fmt.Sprintf("Invalid prefix in %s, wanted %s", req.URL.Path, prefix))

	}
	return strings.TrimPrefix(req.URL.Path, prefix), nil
}

// DoRequest will make the call to the real webhook.
// body is closed.
func DoRequest(ctx context.Context, delegate Delegate, uid typesv1.UID, body io.ReadCloser) *admissionv1.AdmissionResponse {
	// Note it's fine if delegate.CACertPool is nil because that just means
	// we use container root CA.
	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: delegate.CACertPool},
	}
	client := &http.Client{Transport: transCfg}

	proxyReq, err := http.NewRequest("POST", delegate.Service, body)
	proxyReq.Header.Set("Content-Type", "application/json")
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to create request: %s", err)
		return CreateFailResponse(uid, fmt.Sprintf("Failed to post %s", err))
	}
	proxyResp, err := client.Do(proxyReq)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to post: %s", err)
		return CreateFailResponse(uid, fmt.Sprintf("Failed to post %s", err))
	}
	if proxyResp == nil {
		logging.FromContext(ctx).Error("Nil response from proxy")
		return CreateFailResponse(uid, "Nil response from proxy")
	}
	defer proxyResp.Body.Close()

	b, err := io.ReadAll(proxyResp.Body)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to read body of response: %s", err)
		return CreateFailResponse(uid, fmt.Sprintf("Failed to read body of response: %s", err))
	}

	ret := &admissionv1.AdmissionResponse{}
	err = json.Unmarshal(b, ret)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to unmarshal response: %s", err)
		return CreateFailResponse(uid, fmt.Sprintf("Failed to unmarshal body of response: %s", err))
	}
	return ret
}
