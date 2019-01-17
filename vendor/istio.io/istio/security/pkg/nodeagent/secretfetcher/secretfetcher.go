// Copyright 2018 Istio Authors
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

package secretfetcher

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	"istio.io/istio/pkg/log"

	ca "istio.io/istio/security/pkg/nodeagent/caclient"

	caClientInterface "istio.io/istio/security/pkg/nodeagent/caclient/interface"

	"istio.io/istio/security/pkg/nodeagent/model"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	secretResyncPeriod = 15 * time.Second

	// IngressSecretType the type of kubernetes secrets for ingress gateway.
	IngressSecretType = "istio.io/ingress-key-cert"

	// KubeConfigFile the config file name for kubernetes client.
	// Specifies empty file name to use InClusterConfig.
	KubeConfigFile = ""

	// The ID/name for the certificate chain in kubernetes secret.
	ScrtCert = "cert"
	// The ID/name for the k8sKey in kubernetes secret.
	ScrtKey = "key"

	// IngressSecretNameSpace the namespace of kubernetes secrets to watch.
	ingressSecretNameSpace = "INGRESS_GATEWAY_NAMESPACE"
)

// SecretFetcher fetches secret via watching k8s secrets or sending CSR to CA.
type SecretFetcher struct {
	// If UseCaClient is true, use caClient to send CSR to CA.
	UseCaClient bool
	CaClient    caClientInterface.Client

	// Controller and store for secret objects.
	scrtController cache.Controller
	scrtStore      cache.Store

	// secrets maps k8sKey to secrets
	secrets sync.Map

	// Delete all entries containing secretName in SecretCache. Called when K8S secret is deleted.
	DeleteCache func(secretName string)
	// Update all entries containing secretName in SecretCache. Called when K8S secret is updated.
	UpdateCache func(secretName string, ns model.SecretItem)
}

func fatalf(template string, args ...interface{}) {
	if len(args) > 0 {
		log.Errorf(template, args...)
	} else {
		log.Errorf(template)
	}
	os.Exit(-1)
}

// createClientset creates kubernetes client to watch kubernetes secrets.
func createClientset() *kubernetes.Clientset {
	c, err := clientcmd.BuildConfigFromFlags("", KubeConfigFile)
	if err != nil {
		fatalf("Failed to create a config for kubernetes client (error: %s)", err)
	}
	cs, err := kubernetes.NewForConfig(c)
	if err != nil {
		fatalf("Failed to create a clientset (error: %s)", err)
	}
	return cs
}

// NewSecretFetcher returns a pointer to a newly constructed SecretFetcher instance.
func NewSecretFetcher(ingressGatewayAgent bool, endpoint, CAProviderName string, tlsFlag bool,
	tlsRootCert []byte, vaultAddr, vaultRole, vaultAuthPath, vaultSignCsrPath string) (*SecretFetcher, error) {
	ret := &SecretFetcher{}

	if ingressGatewayAgent {
		ret.UseCaClient = false
		cs := createClientset()
		ret.Init(cs.CoreV1())
	} else {
		caClient, err := ca.NewCAClient(endpoint, CAProviderName, tlsFlag, tlsRootCert,
			vaultAddr, vaultRole, vaultAuthPath, vaultSignCsrPath)
		if err != nil {
			log.Errorf("failed to create caClient: %v", err)
			return ret, fmt.Errorf("failed to create caClient")
		}
		ret.UseCaClient = true
		ret.CaClient = caClient
	}

	return ret, nil
}

// Run starts the SecretFetcher until a value is sent to ch.
// Only used when watching kubernetes gateway secrets.
func (sf *SecretFetcher) Run(ch chan struct{}) {
	go sf.scrtController.Run(ch)
}

// Init initializes SecretFetcher to watch kubernetes secrets.
func (sf *SecretFetcher) Init(core corev1.CoreV1Interface) { // nolint:interfacer
	namespace := os.Getenv(ingressSecretNameSpace)
	istioSecretSelector := fields.SelectorFromSet(map[string]string{"type": IngressSecretType}).String()
	scrtLW := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = istioSecretSelector
			return core.Secrets(namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = istioSecretSelector
			return core.Secrets(namespace).Watch(options)
		},
	}
	sf.scrtStore, sf.scrtController =
		cache.NewInformer(scrtLW, &v1.Secret{}, secretResyncPeriod, cache.ResourceEventHandlerFuncs{
			AddFunc:    sf.scrtAdded,
			DeleteFunc: sf.scrtDeleted,
			UpdateFunc: sf.scrtUpdated,
		})
}

func (sf *SecretFetcher) scrtAdded(obj interface{}) {
	scrt, ok := obj.(*v1.Secret)
	if !ok {
		log.Warnf("Failed to convert to secret object: %v", obj)
		return
	}

	t := time.Now()
	resourceName := scrt.GetName()
	// If there is secret with the same resource name, delete that secret now.
	sf.secrets.Delete(resourceName)
	ns := &model.SecretItem{
		ResourceName:     resourceName,
		CertificateChain: scrt.Data[ScrtCert],
		PrivateKey:       scrt.Data[ScrtKey],
		CreatedTime:      t,
		Version:          t.String(),
	}
	sf.secrets.Store(resourceName, *ns)
	log.Debugf("secret %s is added", scrt.GetName())
}

func (sf *SecretFetcher) scrtDeleted(obj interface{}) {
	scrt, ok := obj.(*v1.Secret)
	if !ok {
		log.Warnf("Failed to convert to secret object: %v", obj)
		return
	}

	key := scrt.GetName()
	sf.secrets.Delete(key)
	log.Debugf("secret %s is deleted", scrt.GetName())
	// Delete all cache entries that match the deleted key.
	if sf.DeleteCache != nil {
		sf.DeleteCache(key)
	}
}

func (sf *SecretFetcher) scrtUpdated(oldObj, newObj interface{}) {
	oscrt, ok := oldObj.(*v1.Secret)
	if !ok {
		log.Warnf("Failed to convert to old secret object: %v", oldObj)
		return
	}
	nscrt, ok := newObj.(*v1.Secret)
	if !ok {
		log.Warnf("Failed to convert to new secret object: %v", newObj)
		return
	}

	okey := oscrt.GetName()
	nkey := nscrt.GetName()
	if okey != nkey {
		log.Warnf("Failed to update secret: name does not match (%s vs %s).", okey, nkey)
		return
	}
	if bytes.Equal(oscrt.Data[ScrtCert], nscrt.Data[ScrtCert]) && bytes.Equal(oscrt.Data[ScrtKey], nscrt.Data[ScrtKey]) {
		log.Debugf("secret %s does not change, skip update", okey)
		return
	}
	sf.secrets.Delete(okey)

	t := time.Now()
	ns := &model.SecretItem{
		ResourceName:     nkey,
		CertificateChain: nscrt.Data[ScrtCert],
		PrivateKey:       nscrt.Data[ScrtKey],
		CreatedTime:      t,
		Version:          t.String(),
	}
	sf.secrets.Store(nkey, *ns)
	log.Debugf("secret %s is updated", nscrt.GetName())
	if sf.UpdateCache != nil {
		sf.UpdateCache(nkey, *ns)
	}
}

// FindIngressGatewaySecret returns the secret for a k8sKeyA, or empty secret if no
// secret is present. The ok result indicates whether secret was found.
func (sf *SecretFetcher) FindIngressGatewaySecret(key string) (secret model.SecretItem, ok bool) {
	val, exist := sf.secrets.Load(key)
	if !exist {
		return model.SecretItem{}, false
	}
	e := val.(model.SecretItem)
	return e, true
}

// AddSecret adds obj into local store. Only used for testing.
func (sf *SecretFetcher) AddSecret(obj interface{}) {
	sf.scrtAdded(obj)
}
