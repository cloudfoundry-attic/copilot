// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	sync "sync"

	models "code.cloudfoundry.org/copilot/models"
)

type BackendSet struct {
	GetStub        func(models.DiegoProcessGUID) *models.BackendSet
	getMutex       sync.RWMutex
	getArgsForCall []struct {
		arg1 models.DiegoProcessGUID
	}
	getReturns struct {
		result1 *models.BackendSet
	}
	getReturnsOnCall map[int]struct {
		result1 *models.BackendSet
	}
	GetInternalBackendsStub        func(models.DiegoProcessGUID) *models.BackendSet
	getInternalBackendsMutex       sync.RWMutex
	getInternalBackendsArgsForCall []struct {
		arg1 models.DiegoProcessGUID
	}
	getInternalBackendsReturns struct {
		result1 *models.BackendSet
	}
	getInternalBackendsReturnsOnCall map[int]struct {
		result1 *models.BackendSet
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *BackendSet) Get(arg1 models.DiegoProcessGUID) *models.BackendSet {
	fake.getMutex.Lock()
	ret, specificReturn := fake.getReturnsOnCall[len(fake.getArgsForCall)]
	fake.getArgsForCall = append(fake.getArgsForCall, struct {
		arg1 models.DiegoProcessGUID
	}{arg1})
	fake.recordInvocation("Get", []interface{}{arg1})
	fake.getMutex.Unlock()
	if fake.GetStub != nil {
		return fake.GetStub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	fakeReturns := fake.getReturns
	return fakeReturns.result1
}

func (fake *BackendSet) GetCallCount() int {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	return len(fake.getArgsForCall)
}

func (fake *BackendSet) GetCalls(stub func(models.DiegoProcessGUID) *models.BackendSet) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = stub
}

func (fake *BackendSet) GetArgsForCall(i int) models.DiegoProcessGUID {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	argsForCall := fake.getArgsForCall[i]
	return argsForCall.arg1
}

func (fake *BackendSet) GetReturns(result1 *models.BackendSet) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	fake.getReturns = struct {
		result1 *models.BackendSet
	}{result1}
}

func (fake *BackendSet) GetReturnsOnCall(i int, result1 *models.BackendSet) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	if fake.getReturnsOnCall == nil {
		fake.getReturnsOnCall = make(map[int]struct {
			result1 *models.BackendSet
		})
	}
	fake.getReturnsOnCall[i] = struct {
		result1 *models.BackendSet
	}{result1}
}

func (fake *BackendSet) GetInternalBackends(arg1 models.DiegoProcessGUID) *models.BackendSet {
	fake.getInternalBackendsMutex.Lock()
	ret, specificReturn := fake.getInternalBackendsReturnsOnCall[len(fake.getInternalBackendsArgsForCall)]
	fake.getInternalBackendsArgsForCall = append(fake.getInternalBackendsArgsForCall, struct {
		arg1 models.DiegoProcessGUID
	}{arg1})
	fake.recordInvocation("GetInternalBackends", []interface{}{arg1})
	fake.getInternalBackendsMutex.Unlock()
	if fake.GetInternalBackendsStub != nil {
		return fake.GetInternalBackendsStub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	fakeReturns := fake.getInternalBackendsReturns
	return fakeReturns.result1
}

func (fake *BackendSet) GetInternalBackendsCallCount() int {
	fake.getInternalBackendsMutex.RLock()
	defer fake.getInternalBackendsMutex.RUnlock()
	return len(fake.getInternalBackendsArgsForCall)
}

func (fake *BackendSet) GetInternalBackendsCalls(stub func(models.DiegoProcessGUID) *models.BackendSet) {
	fake.getInternalBackendsMutex.Lock()
	defer fake.getInternalBackendsMutex.Unlock()
	fake.GetInternalBackendsStub = stub
}

func (fake *BackendSet) GetInternalBackendsArgsForCall(i int) models.DiegoProcessGUID {
	fake.getInternalBackendsMutex.RLock()
	defer fake.getInternalBackendsMutex.RUnlock()
	argsForCall := fake.getInternalBackendsArgsForCall[i]
	return argsForCall.arg1
}

func (fake *BackendSet) GetInternalBackendsReturns(result1 *models.BackendSet) {
	fake.getInternalBackendsMutex.Lock()
	defer fake.getInternalBackendsMutex.Unlock()
	fake.GetInternalBackendsStub = nil
	fake.getInternalBackendsReturns = struct {
		result1 *models.BackendSet
	}{result1}
}

func (fake *BackendSet) GetInternalBackendsReturnsOnCall(i int, result1 *models.BackendSet) {
	fake.getInternalBackendsMutex.Lock()
	defer fake.getInternalBackendsMutex.Unlock()
	fake.GetInternalBackendsStub = nil
	if fake.getInternalBackendsReturnsOnCall == nil {
		fake.getInternalBackendsReturnsOnCall = make(map[int]struct {
			result1 *models.BackendSet
		})
	}
	fake.getInternalBackendsReturnsOnCall[i] = struct {
		result1 *models.BackendSet
	}{result1}
}

func (fake *BackendSet) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	fake.getInternalBackendsMutex.RLock()
	defer fake.getInternalBackendsMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *BackendSet) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}
