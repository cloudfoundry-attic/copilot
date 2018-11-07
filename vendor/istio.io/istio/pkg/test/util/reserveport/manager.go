//  Copyright 2018 Istio Authors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package reserveport

import (
	"fmt"
	"sync"
)

type managerImpl struct {
	pool  []ReservedPort
	index int
	mutex sync.Mutex
}

func (m *managerImpl) ReservePort() (ReservedPort, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.index >= len(m.pool) {
		// Re-create the pool.
		var err error
		m.pool, err = allocatePool(poolSize)
		if err != nil {
			return nil, err
		}
		m.index = 0
		// Don't need to free the pool, since all ports have been reserved.
	}

	p := m.pool[m.index]
	if p == nil {
		return nil, fmt.Errorf("attempting to reserve port after manager closed")
	}
	m.pool[m.index] = nil
	m.index++
	return p, nil
}

func (m *managerImpl) Close() (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	pool := m.pool
	m.pool = nil
	m.index = 0
	return freePool(pool)
}
