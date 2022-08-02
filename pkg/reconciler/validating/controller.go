/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validating

import (
	"context"

	vwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/validatingwebhookconfiguration"
	nsinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"

	"github.com/chainguard-dev/admission-sidecar/pkg/filter"
	"github.com/chainguard-dev/admission-sidecar/pkg/proxy"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const queueName = "ProxyAdmissionWebhook"

func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	vwhInformer := vwhinformer.Get(ctx)
	nsInformer := nsinformer.Get(ctx)
	r := &Reconciler{
		delegates:    make(map[string]*proxy.Delegate),
		vwhlister:    vwhInformer.Lister(),
		nslister:     nsInformer.Lister(),
		requireLabel: filter.GetRequireLabel(ctx),
	}

	impl := controller.NewContext(ctx, r, controller.ControllerOptions{
		WorkQueueName: queueName,
		Logger:        logging.FromContext(ctx).Named(queueName),
	})
	vwhInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}
