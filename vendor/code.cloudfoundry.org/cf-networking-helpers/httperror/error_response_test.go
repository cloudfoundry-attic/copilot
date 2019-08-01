package httperror_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/cf-networking-helpers/fakes"
	"code.cloudfoundry.org/cf-networking-helpers/httperror"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ErrorResponse", func() {
	var (
		errorResponse     *httperror.ErrorResponse
		logger            *lagertest.TestLogger
		fakeMetricsSender *fakes.MetricsSender
		resp              *httptest.ResponseRecorder
		err               error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeMetricsSender = &fakes.MetricsSender{}
		errorResponse = &httperror.ErrorResponse{
			MetricsSender: fakeMetricsSender,
		}
		resp = httptest.NewRecorder()
		err = errors.New("potato")
	})

	DescribeTable("when ErrorResponse is called for a given error type",
		func(errorCall string, expectedStatusCode int) {
			switch errorCall {
			case "InternalServerError":
				errorResponse.InternalServerError(logger, resp, err, "description")
			case "BadRequest":
				errorResponse.BadRequest(logger, resp, err, "description")
			case "Forbidden":
				errorResponse.Forbidden(logger, resp, err, "description")
			case "Unauthorized":
				errorResponse.Unauthorized(logger, resp, err, "description")
			case "NotFound":
				errorResponse.NotFound(logger, resp, err, "description")
			case "Conflict":
				errorResponse.Conflict(logger, resp, err, "description")
			case "NotAcceptable":
				errorResponse.NotAcceptable(logger, resp, err, "description")
			default:
				Fail("wrote bad tests")
			}

			Expect(logger).To(gbytes.Say("test.*description.*potato"))
			Expect(resp.Code).To(Equal(expectedStatusCode))
			Expect(resp.Body.String()).To(MatchJSON(`{"error": "description"}`))
			Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
			Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("http_error"))
		},

		Entry("internal server error", "InternalServerError", http.StatusInternalServerError),
		Entry("bad request", "BadRequest", http.StatusBadRequest),
		Entry("forbidden", "Forbidden", http.StatusForbidden),
		Entry("not found", "NotFound", http.StatusNotFound),
		Entry("unauthorized", "Unauthorized", http.StatusUnauthorized),
		Entry("conflict", "Conflict", http.StatusConflict),
		Entry("not acceptable", "NotAcceptable", http.StatusNotAcceptable),
	)

	Context("when a metadata error is supplied", func() {
		It("returns the metadata inside the response", func() {
			metadata := map[string]interface{}{
				"some-metadata": "foo",
			}

			errorResponse.BadRequest(logger, resp, httperror.NewMetadataError(errors.New("potato"), metadata), "description")

			Expect(logger).To(gbytes.Say("test.*description.*potato"))
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
			Expect(resp.Body.String()).To(MatchJSON(`{
				"error": "description",
				"metadata": {
					"some-metadata": "foo"
				}
			}`))
			Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
			Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("http_error"))
		})
	})

	Context("when unauthorized", func() {
		It("returns www-authenticate in header in compliance with RFC 6750", func() {
			errorResponse.Unauthorized(logger, resp, err, "description")

			Expect(resp.Header().Get("www-authenticate")).To(Equal("Bearer"))
		})
	})
})
