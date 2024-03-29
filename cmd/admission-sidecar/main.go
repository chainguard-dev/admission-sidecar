/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"fmt"

	"github.com/chainguard-dev/admission-sidecar/pkg/filter"
	"github.com/chainguard-dev/admission-sidecar/pkg/reconciler/mutating"
	"github.com/chainguard-dev/admission-sidecar/pkg/reconciler/validating"
	"github.com/kelseyhightower/envconfig"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
)

type EnvConfig struct {
	Port         int  `envconfig:"PROXY_PORT" default:"8088"`
	RequireLabel bool `envconfig:"REQUIRE_LABEL" default:"false"`
}

func main() {
	var ec EnvConfig
	err := envconfig.Process("proxy", &ec)
	if err != nil {
		panic(fmt.Sprintf("failed to process env variables: %v", err))
	}

	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "admission-sidecar",
		Port:        ec.Port,
	})
	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx = sharedmain.WithHADisabled(ctx)

	ctx = filter.WithRequireLabel(ctx, ec.RequireLabel)
	logging.FromContext(ctx).Infof("Enforcing only on labeled namespaces: %v", ec.RequireLabel)
	logging.FromContext(ctx).Infof("Starting to listen on %d", ec.Port)
	sharedmain.MainWithConfig(ctx, "admission-sidecar", cfg,
		// NewValidationAdmissionController,
		mutating.NewController,
		// Controller
		validating.NewController,
	)
}
