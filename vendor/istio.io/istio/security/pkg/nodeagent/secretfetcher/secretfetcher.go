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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"

	"istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/log"
	ca "istio.io/istio/security/pkg/nodeagent/caclient"
	caClientInterface "istio.io/istio/security/pkg/nodeagent/caclient/interface"
	"istio.io/istio/security/pkg/nodeagent/model"
)

const (
	secretResyncPeriod = 15 * time.Second

	// The ID/name for the certificate chain in kubernetes generic secret.
	genericScrtCert = "cert"
	// The ID/name for the private key in kubernetes generic secret.
	genericScrtKey = "key"
	// The ID/name for the CA certificate in kubernetes generic secret.
	genericScrtCaCert = "cacert"

	// The ID/name for the certificate chain in kubernetes tls secret.
	tlsScrtCert = "tls.crt"
	// The ID/name for the k8sKey in kubernetes tls secret.
	tlsScrtKey = "tls.key"

	// IngressSecretNameSpace the namespace of kubernetes secrets to watch.
	ingressSecretNameSpace = "INGRESS_GATEWAY_NAMESPACE"

	// IngressGatewaySdsCaSuffix is the suffix of the sds resource name for root CA. All resource
	// names for ingress gateway root certs end with "-cacert".
	IngressGatewaySdsCaSuffix = "-cacert"
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

// NewSecretFetcher returns a pointer to a newly constructed SecretFetcher instance.
func NewSecretFetcher(ingressGatewayAgent bool, endpoint, CAProviderName string, tlsFlag bool,
	tlsRootCert []byte, vaultAddr, vaultRole, vaultAuthPath, vaultSignCsrPath string) (*SecretFetcher, error) {
	ret := &SecretFetcher{}

	if ingressGatewayAgent {
		ret.UseCaClient = false
		cs, err := kube.CreateClientset("", "")
		if err != nil {
			fatalf("Could not create k8s clientset: %v", err)
		}
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
	istioSecretSelector := fields.SelectorFromSet(nil).String()
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

func extractCertAndKey(scrt *v1.Secret) (cert, key, root []byte) {
	if len(scrt.Data[genericScrtCert]) > 0 {
		cert = scrt.Data[genericScrtCert]
		key = scrt.Data[genericScrtKey]
		root = scrt.Data[genericScrtCaCert]
	} else {
		cert = scrt.Data[tlsScrtCert]
		key = scrt.Data[tlsScrtKey]
		root = []byte{}
	}
	return cert, key, root
}

func (sf *SecretFetcher) scrtAdded(obj interface{}) {
	scrt, ok := obj.(*v1.Secret)
	if !ok {
		log.Warnf("Failed to convert to secret object: %v", obj)
		return
	}

	t := time.Now()
	resourceName := scrt.GetName()
	newCert, newKey, newRoot := extractCertAndKey(scrt)
	// If there is secret with the same resource name, delete that secret now.
	sf.secrets.Delete(resourceName)
	ns := &model.SecretItem{
		ResourceName:     resourceName,
		CertificateChain: newCert,
		PrivateKey:       newKey,
		CreatedTime:      t,
		Version:          t.String(),
	}
	sf.secrets.Store(resourceName, *ns)
	log.Debugf("secret %s is added", resourceName)

	rootCertResourceName := resourceName + IngressGatewaySdsCaSuffix
	// If there is root cert secret with the same resource name, delete that secret now.
	sf.secrets.Delete(rootCertResourceName)
	if len(newRoot) > 0 {
		nsRoot := &model.SecretItem{
			ResourceName: rootCertResourceName,
			RootCert:     newRoot,
			CreatedTime:  t,
			Version:      t.String(),
		}
		sf.secrets.Store(rootCertResourceName, *nsRoot)
		log.Debugf("secret %s is added", rootCertResourceName)
	}
}

func (sf *SecretFetcher) scrtDeleted(obj interface{}) {
	scrt, ok := obj.(*v1.Secret)
	if !ok {
		log.Warnf("Failed to convert to secret object: %v", obj)
		return
	}

	key := scrt.GetName()
	sf.secrets.Delete(key)
	log.Debugf("secret %s is deleted", key)
	// Delete all cache entries that match the deleted key.
	if sf.DeleteCache != nil {
		sf.DeleteCache(key)
	}

	rootCertResourceName := key + IngressGatewaySdsCaSuffix
	// If there is root cert secret with the same resource name, delete that secret now.
	sf.secrets.Delete(rootCertResourceName)
	log.Debugf("secret %s is deleted", rootCertResourceName)
	// Delete all cache entries that match the deleted key.
	if sf.DeleteCache != nil {
		sf.DeleteCache(rootCertResourceName)
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

	oldScrtName := oscrt.GetName()
	newScrtName := nscrt.GetName()
	if oldScrtName != newScrtName {
		log.Warnf("Failed to update secret: name does not match (%s vs %s).", oldScrtName, newScrtName)
		return
	}
	oldCert, oldKey, oldRoot := extractCertAndKey(oscrt)
	newCert, newKey, newRoot := extractCertAndKey(nscrt)
	if bytes.Equal(oldCert, newCert) && bytes.Equal(oldKey, newKey) && bytes.Equal(oldRoot, newRoot) {
		log.Debugf("secret %s does not change, skip update", oldScrtName)
		return
	}
	sf.secrets.Delete(oldScrtName)

	t := time.Now()
	ns := &model.SecretItem{
		ResourceName:     newScrtName,
		CertificateChain: newCert,
		PrivateKey:       newKey,
		CreatedTime:      t,
		Version:          t.String(),
	}
	sf.secrets.Store(newScrtName, *ns)
	log.Debugf("secret %s is updated", newScrtName)
	if sf.UpdateCache != nil {
		sf.UpdateCache(newScrtName, *ns)
	}

	rootCertResourceName := newScrtName + IngressGatewaySdsCaSuffix
	// If there is root cert secret with the same resource name, delete that secret now.
	sf.secrets.Delete(rootCertResourceName)
	if len(newRoot) > 0 {
		nsRoot := &model.SecretItem{
			ResourceName: rootCertResourceName,
			RootCert:     newRoot,
			CreatedTime:  t,
			Version:      t.String(),
		}
		sf.secrets.Store(rootCertResourceName, *nsRoot)
		log.Debugf("secret %s is updated", rootCertResourceName)
		if sf.UpdateCache != nil {
			sf.UpdateCache(rootCertResourceName, *nsRoot)
		}
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
