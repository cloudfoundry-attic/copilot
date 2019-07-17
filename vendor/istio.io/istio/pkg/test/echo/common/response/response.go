//  Copyright 2019 Istio Authors
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

package response

import (
	"net/http"
	"strconv"
)

var (
	StatusCodeOK        = strconv.Itoa(http.StatusOK)
	StatusCodeForbidden = strconv.Itoa(http.StatusForbidden)
)

// Field is a list of fields returned in responses from the Echo server.
type Field string

const (
	RequestIDField      Field = "X-Request-Id"
	ServiceVersionField Field = "ServiceVersion"
	ServicePortField    Field = "ServicePort"
	StatusCodeField     Field = "StatusCode"
	HostField           Field = "Host"
	HostnameField       Field = "Hostname"
)
