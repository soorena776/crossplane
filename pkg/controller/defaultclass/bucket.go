/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package defaultclass

import (
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	storagev1alpha1 "github.com/crossplaneio/crossplane/apis/storage/v1alpha1"
)

// BucketController is responsible for adding the default class controller
// for BucketInstance and its corresponding reconciler to the manager with any runtime configuration.
type BucketController struct{}

// SetupWithManager adds a default class controller that reconciles claims
// of kind Bucket to a resource class that declares it as the Bucket
// default
func (c *BucketController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewDefaultClassReconciler(mgr,
		resource.ClaimKind(storagev1alpha1.BucketGroupVersionKind),
		resource.PolicyKind{Singular: storagev1alpha1.BucketPolicyGroupVersionKind, Plural: storagev1alpha1.BucketPolicyListGroupVersionKind},
	)

	name := strings.ToLower(fmt.Sprintf("%s.%s", storagev1alpha1.BucketKind, controllerBaseName))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&storagev1alpha1.Bucket{}).
		WithEventFilter(resource.NewPredicates(resource.NoClassReference())).
		WithEventFilter(resource.NewPredicates(resource.NoManagedResourceReference())).
		Complete(r)
}
