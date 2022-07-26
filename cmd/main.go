/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	// The set of controllers this controller process runs.
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"

	"github.com/chainguard-dev/admission-sidecar/pkg/reconciler/mutating"
	"github.com/chainguard-dev/admission-sidecar/pkg/reconciler/validating"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{}

var callbacks = map[schema.GroupVersionKind]validation.Callback{}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.admission-sidecar.chainguard.dev",

		// The path on which to serve the webhook.
		"/admit",

		// The resources to validate.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			// Here is where you would infuse the context with state
			// (e.g. attach a store with configmap data)
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func main() {
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "admission-sidecar",
		Port:        8443,
		//SecretName:  "webhook-certs",
	})

	logging.FromContext(ctx).Infof("SYSTEM_NAMESPACE IS %s", system.Namespace())
	logging.FromContext(ctx).Infof("SYSTEM_NAMESPACE ENV KEY IS %s", system.NamespaceEnvKey)
	cfg := injection.ParseAndGetRESTConfigOrDie()

	// Increase our client-side rate limits.
	cfg.QPS = 100 * cfg.QPS
	cfg.Burst = 100 * cfg.Burst
	ctx = sharedmain.WithHADisabled(ctx)

	sharedmain.MainWithConfig(ctx, "admission-sidecar", cfg,
		// Webhook stuff.
		certificates.NewController,
		NewValidationAdmissionController,
		mutating.NewController,
		// Controller
		validating.NewController,
	)
}
