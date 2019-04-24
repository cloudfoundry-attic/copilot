// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"code.cloudfoundry.org/copilot/certs"
)

type Locator struct {
	LocateStub        func() ([]certs.PemInfo, error)
	locateMutex       sync.RWMutex
	locateArgsForCall []struct {
	}
	locateReturns struct {
		result1 []certs.PemInfo
		result2 error
	}
	locateReturnsOnCall map[int]struct {
		result1 []certs.PemInfo
		result2 error
	}
	StowStub        func() error
	stowMutex       sync.RWMutex
	stowArgsForCall []struct {
	}
	stowReturns struct {
		result1 error
	}
	stowReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *Locator) Locate() ([]certs.PemInfo, error) {
	fake.locateMutex.Lock()
	ret, specificReturn := fake.locateReturnsOnCall[len(fake.locateArgsForCall)]
	fake.locateArgsForCall = append(fake.locateArgsForCall, struct {
	}{})
	fake.recordInvocation("Locate", []interface{}{})
	fake.locateMutex.Unlock()
	if fake.LocateStub != nil {
		return fake.LocateStub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.locateReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *Locator) LocateCallCount() int {
	fake.locateMutex.RLock()
	defer fake.locateMutex.RUnlock()
	return len(fake.locateArgsForCall)
}

func (fake *Locator) LocateCalls(stub func() ([]certs.PemInfo, error)) {
	fake.locateMutex.Lock()
	defer fake.locateMutex.Unlock()
	fake.LocateStub = stub
}

func (fake *Locator) LocateReturns(result1 []certs.PemInfo, result2 error) {
	fake.locateMutex.Lock()
	defer fake.locateMutex.Unlock()
	fake.LocateStub = nil
	fake.locateReturns = struct {
		result1 []certs.PemInfo
		result2 error
	}{result1, result2}
}

func (fake *Locator) LocateReturnsOnCall(i int, result1 []certs.PemInfo, result2 error) {
	fake.locateMutex.Lock()
	defer fake.locateMutex.Unlock()
	fake.LocateStub = nil
	if fake.locateReturnsOnCall == nil {
		fake.locateReturnsOnCall = make(map[int]struct {
			result1 []certs.PemInfo
			result2 error
		})
	}
	fake.locateReturnsOnCall[i] = struct {
		result1 []certs.PemInfo
		result2 error
	}{result1, result2}
}

func (fake *Locator) Stow() error {
	fake.stowMutex.Lock()
	ret, specificReturn := fake.stowReturnsOnCall[len(fake.stowArgsForCall)]
	fake.stowArgsForCall = append(fake.stowArgsForCall, struct {
	}{})
	fake.recordInvocation("Stow", []interface{}{})
	fake.stowMutex.Unlock()
	if fake.StowStub != nil {
		return fake.StowStub()
	}
	if specificReturn {
		return ret.result1
	}
	fakeReturns := fake.stowReturns
	return fakeReturns.result1
}

func (fake *Locator) StowCallCount() int {
	fake.stowMutex.RLock()
	defer fake.stowMutex.RUnlock()
	return len(fake.stowArgsForCall)
}

func (fake *Locator) StowCalls(stub func() error) {
	fake.stowMutex.Lock()
	defer fake.stowMutex.Unlock()
	fake.StowStub = stub
}

func (fake *Locator) StowReturns(result1 error) {
	fake.stowMutex.Lock()
	defer fake.stowMutex.Unlock()
	fake.StowStub = nil
	fake.stowReturns = struct {
		result1 error
	}{result1}
}

func (fake *Locator) StowReturnsOnCall(i int, result1 error) {
	fake.stowMutex.Lock()
	defer fake.stowMutex.Unlock()
	fake.StowStub = nil
	if fake.stowReturnsOnCall == nil {
		fake.stowReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.stowReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *Locator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.locateMutex.RLock()
	defer fake.locateMutex.RUnlock()
	fake.stowMutex.RLock()
	defer fake.stowMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *Locator) recordInvocation(key string, args []interface{}) {
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

var _ certs.Librarian = new(Locator)
