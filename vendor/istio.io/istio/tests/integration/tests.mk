#-----------------------------------------------------------------------------
# Target: test.integration.*
#-----------------------------------------------------------------------------

# The following flags (in addition to ${V}) can be specified on the command-line, or the environment. This
# is primarily used by the CI systems.

# $(CI) specifies that the test is running in a CI system. This enables CI specific logging.
_INTEGRATION_TEST_CIMODE_FLAG =
_INTEGRATION_TEST_PULL_POLICY = Always
ifneq ($(CI),)
	_INTEGRATION_TEST_CIMODE_FLAG = --istio.test.ci
	_INTEGRATION_TEST_PULL_POLICY = IfNotPresent      # Using Always in CircleCI causes pull issues as images are local.
endif

# In Prow, ARTIFACTS_DIR points to the location where Prow captures the artifacts from the tests
INTEGRATION_TEST_WORKDIR =
ifneq ($(ARTIFACTS_DIR),)
	INTEGRATION_TEST_WORKDIR = ${ARTIFACTS_DIR}
endif

_INTEGRATION_TEST_INGRESS_FLAG =
ifeq (${TEST_ENV},minikube)
    _INTEGRATION_TEST_INGRESS_FLAG = --istio.test.kube.minikube
else ifeq (${TEST_ENV},minikube-none)
    _INTEGRATION_TEST_INGRESS_FLAG = --istio.test.kube.minikube
endif


# $(INTEGRATION_TEST_WORKDIR) specifies the working directory for the tests. If not specified, then a
# temporary folder is used.
_INTEGRATION_TEST_WORKDIR_FLAG =
ifneq ($(INTEGRATION_TEST_WORKDIR),)
    _INTEGRATION_TEST_WORKDIR_FLAG = --istio.test.work_dir $(INTEGRATION_TEST_WORKDIR)
endif

# $(INTEGRATION_TEST_KUBECONFIG) specifies the kube config file to be used. If not specified, then
# ~/.kube/config is used.
# TODO: This probably needs to be more intelligent and take environment variables into account.
INTEGRATION_TEST_KUBECONFIG = ~/.kube/config
ifneq ($(KUBECONFIG),)
    INTEGRATION_TEST_KUBECONFIG = $(KUBECONFIG)
endif

# Generate integration test targets for kubernetes environment.
test.integration.%.kube:
	$(GO) test -p 1 ${T} ./tests/integration/$*/... ${_INTEGRATION_TEST_WORKDIR_FLAG} ${_INTEGRATION_TEST_CIMODE_FLAG} -timeout 30m \
	--istio.test.env kube \
	--istio.test.kube.config ${INTEGRATION_TEST_KUBECONFIG} \
	--istio.test.hub=${HUB} \
	--istio.test.tag=${TAG} \
	--istio.test.pullpolicy=${_INTEGRATION_TEST_PULL_POLICY} \
	${_INTEGRATION_TEST_INGRESS_FLAG}

# Generate integration test targets for local environment.
test.integration.%:
	$(GO) test -p 1 ${T} ./tests/integration/$*/... --istio.test.env native

JUNIT_UNIT_TEST_XML ?= $(ISTIO_OUT)/junit_unit-tests.xml
JUNIT_REPORT = $(shell which go-junit-report 2> /dev/null || echo "${ISTIO_BIN}/go-junit-report")

# TODO: Exclude examples and qualification since they are very flaky.
TEST_PACKAGES = $(shell go list ./tests/integration/... | grep -v /qualification | grep -v /examples)

# Generate presubmit integration test targets for each component in kubernetes environment
test.integration.%.kube.presubmit: | $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_UNIT_TEST_XML))
	set -o pipefail; \
	$(GO) test -p 1 ${T} ./tests/integration/$*/... ${_INTEGRATION_TEST_WORKDIR_FLAG} ${_INTEGRATION_TEST_CIMODE_FLAG} -timeout 30m \
    --istio.test.select -postsubmit,-flaky \
	--istio.test.env kube \
	--istio.test.kube.config ${INTEGRATION_TEST_KUBECONFIG} \
	--istio.test.hub=${HUB} \
	--istio.test.tag=${TAG} \
	--istio.test.pullpolicy=${_INTEGRATION_TEST_PULL_POLICY} \
	${_INTEGRATION_TEST_INGRESS_FLAG} \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_UNIT_TEST_XML))

# Generate presubmit integration test targets for each component in local environment.
test.integration.%.local.presubmit: | $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_UNIT_TEST_XML))
	set -o pipefail; \
	$(GO) test -p 1 ${T} ./tests/integration/$*/... \
	--istio.test.env native --istio.test.select -postsubmit,-flaky \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_UNIT_TEST_XML))

# All integration tests targeting local environment.
.PHONY: test.integration.local
test.integration.local: | $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_UNIT_TEST_XML))
	set -o pipefail; \
	$(GO) test -p 1 ${T} ${TEST_PACKAGES} --istio.test.env native \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_UNIT_TEST_XML))

# Presubmit integration tests targeting local environment.
.PHONY: test.integration.local.presubmit
test.integration.local.presubmit: | $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_UNIT_TEST_XML))
	set -o pipefail; \
	$(GO) test -p 1 ${T} ${TEST_PACKAGES} --istio.test.env native --istio.test.select -postsubmit,-flaky \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_UNIT_TEST_XML))

# All integration tests targeting Kubernetes environment.
.PHONY: test.integration.kube
test.integration.kube: | $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_UNIT_TEST_XML))
	set -o pipefail; \
	$(GO) test -p 1 ${T} ${TEST_PACKAGES} ${_INTEGRATION_TEST_WORK_DIR_FLAG} ${_INTEGRATION_TEST_CIMODE_FLAG} -timeout 30m \
	--istio.test.env kube \
	--istio.test.kube.config ${INTEGRATION_TEST_KUBECONFIG} \
	--istio.test.hub=${HUB} \
	--istio.test.tag=${TAG} \
	--istio.test.pullpolicy=${_INTEGRATION_TEST_PULL_POLICY} \
	${_INTEGRATION_TEST_INGRESS_FLAG} \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_UNIT_TEST_XML))

# Presubmit integration tests targeting Kubernetes environment.
.PHONY: test.integration.kube.presubmit
test.integration.kube.presubmit: | $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_UNIT_TEST_XML))
	set -o pipefail; \
	$(GO) test -p 1 ${T} ${TEST_PACKAGES} ${_INTEGRATION_TEST_WORK_DIR_FLAG} ${_INTEGRATION_TEST_CIMODE_FLAG} -timeout 30m \
    --istio.test.select -postsubmit,-flaky \
 	--istio.test.env kube \
	--istio.test.kube.config ${INTEGRATION_TEST_KUBECONFIG} \
	--istio.test.hub=${HUB} \
	--istio.test.tag=${TAG} \
	--istio.test.pullpolicy=${_INTEGRATION_TEST_PULL_POLICY} \
	${_INTEGRATION_TEST_INGRESS_FLAG} \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_UNIT_TEST_XML))

# Integration tests that detect race condition for native environment.
.PHONY: test.integration.race.native
test.integration.race.native: | $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_UNIT_TEST_XML))
	set -o pipefail; \
	$(GO) test -race -p 1 ${T} ${TEST_PACKAGES} -timeout 120m \
	--istio.test.env native \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_UNIT_TEST_XML))
