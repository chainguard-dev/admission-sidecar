/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package mutating

import (
	"context"

	mwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/mutatingwebhookconfiguration"
	nsinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"

	"github.com/chainguard-dev/admission-sidecar/pkg/filter"
	"github.com/chainguard-dev/admission-sidecar/pkg/proxy"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const queueName = "ProxyMutatingWebhook"

func NewController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	mwhInformer := mwhinformer.Get(ctx)
	nsInformer := nsinformer.Get(ctx)
	r := &Reconciler{
		delegates:    make(map[string]*proxy.Delegate),
		mwhlister:    mwhInformer.Lister(),
		nslister:     nsInformer.Lister(),
		requireLabel: filter.GetRequireLabel(ctx),
	}
	impl := controller.NewContext(ctx, r, controller.ControllerOptions{
		WorkQueueName: queueName,
		Logger:        logging.FromContext(ctx).Named(queueName),
	})

	_, _ = mwhInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}
