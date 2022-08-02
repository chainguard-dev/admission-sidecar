/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package filter

import (
	"context"

	v1 "k8s.io/api/core/v1"
)

// Label that a namespace must have or proxy will just return pass for it.
const InclusionLabel = "proxy.chainguard.dev/include"
const InclusionValue = "true"

// ShouldProxy checks the namespace for labels to see if the namespace is
// labeled for inclusion.
func ShouldProxy(ctx context.Context, ns *v1.Namespace) bool {
	if !GetRequireLabel(ctx) {
		return true
	}
	if label, ok := ns.Labels[InclusionLabel]; ok {
		return label == InclusionValue
	}
	return false
}

// requireLabelKey is used as the key for associating whether label
// is required on a namespace or not.
type requireLabelKey struct{}

// WithRequireLabel sets whether a label is required for the proxy
// to work.
func WithRequireLabel(ctx context.Context, requireLabel bool) context.Context {
	return context.WithValue(ctx, requireLabelKey{}, requireLabel)
}

// GetOptions retrieves webhook.Options associated with the
// given context via WithOptions (above).
func GetRequireLabel(ctx context.Context) bool {
	v := ctx.Value(requireLabelKey{})
	if v == nil {
		return false
	}
	return v.(bool)
}
