/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package mutating

import (
	"context"
	"fmt"
	"sync"

	"github.com/chainguard-dev/admission-sidecar/pkg/filter"
	"github.com/chainguard-dev/admission-sidecar/pkg/proxy"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admissionregistration/v1"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	nslisters "k8s.io/client-go/listers/core/v1"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/webhook"
)

const mutatePrefix = "/mutate/"

// Reconciler implements the meta AdmissionController
type Reconciler struct {
	webhook.StatelessAdmissionImpl
	mwhlister    admissionlisters.MutatingWebhookConfigurationLister
	nslister     nslisters.NamespaceLister
	requireLabel bool

	m         sync.Mutex
	delegates map[string]*proxy.Delegate
}

var _ controller.Reconciler = (*Reconciler)(nil)
var _ webhook.AdmissionController = (*Reconciler)(nil)

// Reconcile adds Client information to our map for each of the
// MutatingWebhookConfiguration so that the Admit can call them
// as necessary.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	mwh, err := r.mwhlister.Get(key)
	if err != nil {
		return err
	}

	for i := range mwh.Webhooks {
		if err := r.addDelegate(ctx, mwh.Webhooks[i].Name, mwh.Webhooks[i].ClientConfig); err != nil {
			logging.FromContext(ctx).Errorf("Failed to add delegate: %s", err)
			return err
		}
	}
	return nil
}
func (r *Reconciler) addDelegate(ctx context.Context, name string, clientConfig v1.WebhookClientConfig) error {
	r.m.Lock()
	defer r.m.Unlock()
	delegate, err := proxy.WebhookClientConfigToURLAndCert(clientConfig)
	if err != nil {
		return err
	}
	r.delegates[name] = delegate
	if clientConfig.Service != nil {
		logging.FromContext(ctx).Infof("Added %s => Service: %+v", name, clientConfig.Service)
	} else {
		logging.FromContext(ctx).Infof("Added %s => URL: %s", name, *clientConfig.URL)
	}
	return nil
}

func (r *Reconciler) getDelegate(name string) *proxy.Delegate {
	r.m.Lock()
	defer r.m.Unlock()
	return r.delegates[name]
}

func (r *Reconciler) Path() string {
	return mutatePrefix
}

// Admit implements webhook.AdmissionController
func (r *Reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	ctx = filter.WithRequireLabel(ctx, r.requireLabel)
	if request.Namespace != "" {
		ns, err := r.nslister.Get(request.Namespace)
		if err != nil {
			return proxy.CreateFailResponse(request.UID, fmt.Sprintf("Failed to get namespace %s %s", request.Namespace, err))
		}
		if !filter.ShouldProxy(ctx, ns) {
			logging.FromContext(ctx).Debugf("Namespace %s not labeled for inclusion, letting through", request.Namespace)
			return proxy.CreateAllowResponse(request.UID)
		}
	}

	req := apis.GetHTTPRequest(ctx)
	hook, response := proxy.GetHookName(ctx, mutatePrefix, request.UID, req)
	if response != nil {
		return response
	}
	delegate := r.getDelegate(hook)
	if delegate == nil || delegate.Service == "" {
		logging.FromContext(ctx).Errorf("No handler found for %s %s", req.URL.Path, hook)
		return proxy.CreateFailResponse(request.UID, fmt.Sprintf("No handler found for %s %s", req.URL.Path, hook))
	}
	logging.FromContext(ctx).Errorf("Doing a proxy request to delegate %s : %s", hook, delegate.Service)
	return proxy.DoRequest(ctx, *delegate, request)
}
