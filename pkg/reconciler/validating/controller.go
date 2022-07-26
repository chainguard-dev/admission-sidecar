/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validating

import (
	"context"

	vwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/validatingwebhookconfiguration"
	vwhreconciler "knative.dev/pkg/client/injection/kube/reconciler/admissionregistration/v1/validatingwebhookconfiguration"

	"github.com/chainguard-dev/admission-sidecar/pkg/proxy"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	vwhInformer := vwhinformer.Get(ctx)
	r := &Reconciler{
		delegates: make(map[string]*proxy.Delegate),
		vwhlister: vwhInformer.Lister(),
	}
	impl := vwhreconciler.NewImpl(ctx, r)
	vwhInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}
