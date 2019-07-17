// Copyright 2019 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mock

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	admissionregistrationv1alpha1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	appsv1beta1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	appsv1beta2 "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	auditregistrationv1alpha1 "k8s.io/client-go/kubernetes/typed/auditregistration/v1alpha1"
	authenticationv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authenticationv1beta1 "k8s.io/client-go/kubernetes/typed/authentication/v1beta1"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	authorizationv1beta1 "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
	autoscalingv1 "k8s.io/client-go/kubernetes/typed/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/client-go/kubernetes/typed/autoscaling/v2beta1"
	autoscalingv2beta2 "k8s.io/client-go/kubernetes/typed/autoscaling/v2beta2"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	batchv1beta1 "k8s.io/client-go/kubernetes/typed/batch/v1beta1"
	batchv2alpha1 "k8s.io/client-go/kubernetes/typed/batch/v2alpha1"
	certificatesv1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	coordinationv1beta1 "k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	eventsv1beta1 "k8s.io/client-go/kubernetes/typed/events/v1beta1"
	extensionsv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	networkingv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	policyv1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacv1alpha1 "k8s.io/client-go/kubernetes/typed/rbac/v1alpha1"
	rbacv1beta1 "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
	schedulingv1alpha1 "k8s.io/client-go/kubernetes/typed/scheduling/v1alpha1"
	schedulingv1beta1 "k8s.io/client-go/kubernetes/typed/scheduling/v1beta1"
	settingsv1alpha1 "k8s.io/client-go/kubernetes/typed/settings/v1alpha1"
	storagev1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	storagev1alpha1 "k8s.io/client-go/kubernetes/typed/storage/v1alpha1"
	storagev1beta1 "k8s.io/client-go/kubernetes/typed/storage/v1beta1"
)

var _ kubernetes.Interface = &kubeInterface{}

type kubeInterface struct {
	core corev1.CoreV1Interface
}

// newKubeInterface returns a lightweight fake that implements kubernetes.Interface. Only implements a portion of the
// interface that is used by galley tests. The clientset provided in the kubernetes/fake package has proven poor
// for benchmarking because of the amount of garbage created and spurious channel full panics due to the fact that
// the fake watch channels are sized at 100 items.
func newKubeInterface() kubernetes.Interface {
	return &kubeInterface{
		core: &corev1Impl{
			nodes:      newNodeInterface(),
			pods:       newPodInterface(),
			services:   newServiceInterface(),
			endpoints:  newEndpointsInterface(),
			namespaces: newNamespaceInterface(),
		},
	}
}

func (c *kubeInterface) CoreV1() corev1.CoreV1Interface {
	return c.core
}

func (c *kubeInterface) Discovery() discovery.DiscoveryInterface {
	panic("not implemented")
}

func (c *kubeInterface) AuditregistrationV1alpha1() auditregistrationv1alpha1.AuditregistrationV1alpha1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Auditregistration() auditregistrationv1alpha1.AuditregistrationV1alpha1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AutoscalingV2beta2() autoscalingv2beta2.AutoscalingV2beta2Interface {
	panic("not implemented")
}

func (c *kubeInterface) CoordinationV1beta1() coordinationv1beta1.CoordinationV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Coordination() coordinationv1beta1.CoordinationV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AdmissionregistrationV1alpha1() admissionregistrationv1alpha1.AdmissionregistrationV1alpha1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AdmissionregistrationV1beta1() admissionregistrationv1beta1.AdmissionregistrationV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Admissionregistration() admissionregistrationv1beta1.AdmissionregistrationV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AppsV1beta1() appsv1beta1.AppsV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AppsV1beta2() appsv1beta2.AppsV1beta2Interface {
	panic("not implemented")
}

func (c *kubeInterface) AppsV1() appsv1.AppsV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Apps() appsv1.AppsV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AuthenticationV1() authenticationv1.AuthenticationV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Authentication() authenticationv1.AuthenticationV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AuthenticationV1beta1() authenticationv1beta1.AuthenticationV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AuthorizationV1() authorizationv1.AuthorizationV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Authorization() authorizationv1.AuthorizationV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AuthorizationV1beta1() authorizationv1beta1.AuthorizationV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AutoscalingV1() autoscalingv1.AutoscalingV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Autoscaling() autoscalingv1.AutoscalingV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) AutoscalingV2beta1() autoscalingv2beta1.AutoscalingV2beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) BatchV1() batchv1.BatchV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Batch() batchv1.BatchV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) BatchV1beta1() batchv1beta1.BatchV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) BatchV2alpha1() batchv2alpha1.BatchV2alpha1Interface {
	panic("not implemented")
}

func (c *kubeInterface) CertificatesV1beta1() certificatesv1beta1.CertificatesV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Certificates() certificatesv1beta1.CertificatesV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Core() corev1.CoreV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) EventsV1beta1() eventsv1beta1.EventsV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Events() eventsv1beta1.EventsV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) ExtensionsV1beta1() extensionsv1beta1.ExtensionsV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Extensions() extensionsv1beta1.ExtensionsV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) NetworkingV1() networkingv1.NetworkingV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Networking() networkingv1.NetworkingV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) PolicyV1beta1() policyv1beta1.PolicyV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Policy() policyv1beta1.PolicyV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) RbacV1() rbacv1.RbacV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Rbac() rbacv1.RbacV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) RbacV1beta1() rbacv1beta1.RbacV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) RbacV1alpha1() rbacv1alpha1.RbacV1alpha1Interface {
	panic("not implemented")
}

func (c *kubeInterface) SchedulingV1alpha1() schedulingv1alpha1.SchedulingV1alpha1Interface {
	panic("not implemented")
}

func (c *kubeInterface) SchedulingV1beta1() schedulingv1beta1.SchedulingV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Scheduling() schedulingv1beta1.SchedulingV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) SettingsV1alpha1() settingsv1alpha1.SettingsV1alpha1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Settings() settingsv1alpha1.SettingsV1alpha1Interface {
	panic("not implemented")
}

func (c *kubeInterface) StorageV1beta1() storagev1beta1.StorageV1beta1Interface {
	panic("not implemented")
}

func (c *kubeInterface) StorageV1() storagev1.StorageV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) Storage() storagev1.StorageV1Interface {
	panic("not implemented")
}

func (c *kubeInterface) StorageV1alpha1() storagev1alpha1.StorageV1alpha1Interface {
	panic("not implemented")
}
