/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package mutating

import (
	"context"

	mwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/mutatingwebhookconfiguration"

	"github.com/chainguard-dev/admission-sidecar/pkg/proxy"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const queueName = "ProxyMutatingWebhook"

func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	mwhInformer := mwhinformer.Get(ctx)
	r := &Reconciler{
		delegates: make(map[string]*proxy.Delegate),
		mwhlister: mwhInformer.Lister(),
	}
	impl := controller.NewContext(ctx, r, controller.ControllerOptions{
		WorkQueueName: queueName,
		Logger:        logging.FromContext(ctx).Named(queueName),
	})

	mwhInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}
