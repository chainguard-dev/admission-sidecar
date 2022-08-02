/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package filter

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestShouldProxyRequireLabel(t *testing.T) {
	tests := []struct {
		name  string
		label string
		want  bool
	}{{
		name:  "no label",
		label: "",
	}, {
		name:  "label wrong",
		label: "nope",
	}, {
		name:  "label right",
		label: "true",
		want:  true,
	}}
	for _, tc := range tests {
		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		if tc.label != "" {
			ns.ObjectMeta.Labels = map[string]string{InclusionLabel: tc.label}
		}
		if got := ShouldProxy(WithRequireLabel(context.Background(), true), ns); tc.want != got {
			t.Errorf("%q want %v got %v", tc.name, tc.want, got)
		}
	}
}

func TestShouldProxyDoNotRequireLabel(t *testing.T) {
	tests := []struct {
		name  string
		label string
		want  bool
	}{{
		name:  "no label",
		label: "",
		want:  true,
	}, {
		name:  "label wrong",
		label: "nope",
		want:  true,
	}, {
		name:  "label right",
		label: "true",
		want:  true,
	}}
	for _, tc := range tests {
		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		if tc.label != "" {
			ns.ObjectMeta.Labels = map[string]string{InclusionLabel: tc.label}
		}
		if got := ShouldProxy(WithRequireLabel(context.Background(), false), ns); tc.want != got {
			t.Errorf("%q want %v got %v", tc.name, tc.want, got)
		}
		// Also check if context is nil.
		if got := ShouldProxy(context.Background(), ns); tc.want != got {
			t.Errorf("%q want %v got %v", tc.name, tc.want, got)
		}
	}
}

func TestRequireLabel(t *testing.T) {
	tests := []struct {
		in   *bool
		want bool
	}{{
		in:   ptr.Bool(true),
		want: true,
	}, {
		in:   ptr.Bool(false),
		want: false,
	}, {
		want: false,
	}}
	for _, tc := range tests {
		ctx := context.Background()
		if tc.in != nil {
			ctx = WithRequireLabel(ctx, *tc.in)
		}
		if got := GetRequireLabel(ctx); tc.want != got {
			t.Errorf("with %v wanted %v got %v", *tc.in, tc.want, got)
		}
	}
}
