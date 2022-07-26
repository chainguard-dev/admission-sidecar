/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validating

import (
	"context"
	"fmt"
	"sync"

	"github.com/chainguard-dev/admission-sidecar/pkg/proxy"
	vwhreconciler "knative.dev/pkg/client/injection/kube/reconciler/admissionregistration/v1/validatingwebhookconfiguration"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admissionregistration/v1"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/webhook"
)

const admitPrefix = "/admit/"

// Reconciler implements the meta AdmissionController
type Reconciler struct {
	vwhlister admissionlisters.ValidatingWebhookConfigurationLister

	m         sync.Mutex
	delegates map[string]*proxy.Delegate
}

var _ vwhreconciler.Interface = (*Reconciler)(nil)
var _ webhook.AdmissionController = (*Reconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *Reconciler) ReconcileKind(ctx context.Context, vwh *v1.ValidatingWebhookConfiguration) reconciler.Event {
	// Ensure our map reflects any updates
	for i := range vwh.Webhooks {
		if err := r.addDelegate(vwh.Webhooks[i].Name, vwh.Webhooks[i].ClientConfig); err != nil {
			logging.FromContext(ctx).Errorf("Failed to add delegate: %s", err)
			return err
		}
	}

	r.m.Lock()
	defer r.m.Unlock()
	for k, v := range r.delegates {
		logging.FromContext(ctx).Infof("Have %s => %+v", k, v)
	}
	return nil
}

func (r *Reconciler) addDelegate(name string, clientConfig v1.WebhookClientConfig) error {
	r.m.Lock()
	defer r.m.Unlock()
	delegate, err := proxy.WebhookClientConfigToURLAndCert(clientConfig)
	if err != nil {
		return err
	}
	r.delegates[name] = delegate
	return nil
}

func (r *Reconciler) getDelegate(name string) *proxy.Delegate {
	r.m.Lock()
	defer r.m.Unlock()
	return r.delegates[name]
}

func (r *Reconciler) Path() string {
	return admitPrefix
}

// Admit implements webhook.AdmissionController
func (r *Reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	req := apis.GetHTTPRequest(ctx)
	hook, response := proxy.GetHookName(ctx, admitPrefix, request.UID, req)
	if response != nil {
		return response
	}
	delegate := r.getDelegate(hook)
	if delegate == nil || delegate.Service == "" {
		logging.FromContext(ctx).Errorf("No handler found for %s %s", req.URL.Path, hook)
		return proxy.CreateFailResponse(request.UID, fmt.Sprintf("No handler found for %s %s", req.URL.Path, hook))
	}
	return proxy.DoRequest(ctx, *delegate, request.UID, req.Body)
}
