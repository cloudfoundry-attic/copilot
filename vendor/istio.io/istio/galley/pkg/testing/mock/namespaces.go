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
	"fmt"
	"sync"

	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ corev1.NamespaceInterface = &namespaceImpl{}

type namespaceImpl struct {
	mux        sync.Mutex
	namespaces map[string]*apicorev1.Namespace
	watches    Watches
}

func newNamespaceInterface() corev1.NamespaceInterface {
	return &namespaceImpl{
		namespaces: make(map[string]*apicorev1.Namespace),
	}
}

func (n *namespaceImpl) Create(obj *apicorev1.Namespace) (*apicorev1.Namespace, error) {
	n.mux.Lock()
	defer n.mux.Unlock()

	n.namespaces[obj.Name] = obj

	n.watches.Send(watch.Event{
		Type:   watch.Added,
		Object: obj,
	})
	return obj, nil
}

func (n *namespaceImpl) Update(obj *apicorev1.Namespace) (*apicorev1.Namespace, error) {
	n.mux.Lock()
	defer n.mux.Unlock()

	n.namespaces[obj.Name] = obj

	n.watches.Send(watch.Event{
		Type:   watch.Modified,
		Object: obj,
	})
	return obj, nil
}

func (n *namespaceImpl) Delete(name string, options *metav1.DeleteOptions) error {
	n.mux.Lock()
	defer n.mux.Unlock()

	obj := n.namespaces[name]
	if obj == nil {
		return fmt.Errorf("unable to delete namespace %s", name)
	}

	delete(n.namespaces, name)

	n.watches.Send(watch.Event{
		Type:   watch.Deleted,
		Object: obj,
	})
	return nil
}

func (n *namespaceImpl) List(opts metav1.ListOptions) (*apicorev1.NamespaceList, error) {
	n.mux.Lock()
	defer n.mux.Unlock()

	out := &apicorev1.NamespaceList{}

	for _, v := range n.namespaces {
		out.Items = append(out.Items, *v)
	}

	return out, nil
}

func (n *namespaceImpl) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	n.mux.Lock()
	defer n.mux.Unlock()

	w := NewWatch()
	n.watches = append(n.watches, w)

	// Send add events for all current resources.
	for _, namespace := range n.namespaces {
		w.Send(watch.Event{
			Type:   watch.Added,
			Object: namespace,
		})
	}

	return w, nil
}

func (n *namespaceImpl) UpdateStatus(*apicorev1.Namespace) (*apicorev1.Namespace, error) {
	panic("not implemented")
}

func (n *namespaceImpl) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("not implemented")
}

func (n *namespaceImpl) Get(name string, options metav1.GetOptions) (*apicorev1.Namespace, error) {
	panic("not implemented")
}

func (n *namespaceImpl) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *apicorev1.Namespace, err error) {
	panic("not implemented")
}

func (n *namespaceImpl) PatchStatus(namespaceName string, data []byte) (*apicorev1.Namespace, error) {
	panic("not implemented")
}

func (n *namespaceImpl) Finalize(item *apicorev1.Namespace) (*apicorev1.Namespace, error) {
	panic("not implemented")
}
