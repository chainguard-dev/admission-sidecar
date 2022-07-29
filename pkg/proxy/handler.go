/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"bytes"
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
func DoRequest(ctx context.Context, delegate Delegate, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	reviewRequest := &admissionv1.AdmissionReview{Request: request}
	body, err := json.Marshal(reviewRequest)
	if err != nil {
		return CreateFailResponse(request.UID, fmt.Sprintf("failed to marshal outgoing AdmissionReview: %s", err))
	}
	// Note it's fine if delegate.CACertPool is nil because that just means
	// we use container root CA.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: delegate.CACertPool},
		},
	}

	proxyReq, err := http.NewRequest(http.MethodPost, delegate.Service, bytes.NewBuffer(body))
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to create request: %s", err)
		return CreateFailResponse(request.UID, fmt.Sprintf("Failed to post %s", err))
	}
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyResp, err := client.Do(proxyReq)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to post: %s", err)
		return CreateFailResponse(request.UID, fmt.Sprintf("Failed to post %s", err))
	}
	if proxyResp == nil {
		logging.FromContext(ctx).Error("Nil response from proxy")
		return CreateFailResponse(request.UID, "Nil response from proxy")
	}
	defer proxyResp.Body.Close()

	b, err := io.ReadAll(proxyResp.Body)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to read body of response: %s", err)
		return CreateFailResponse(request.UID, fmt.Sprintf("Failed to read body of response: %s", err))
	}

	ret := &admissionv1.AdmissionReview{}
	err = json.Unmarshal(b, ret)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to unmarshal response: %s\n%s", err, b)
		return CreateFailResponse(request.UID, fmt.Sprintf("Failed to unmarshal body of response: %s", err))
	}
	logging.FromContext(ctx).Errorf("Got back: %s", b)
	return ret.Response
}
