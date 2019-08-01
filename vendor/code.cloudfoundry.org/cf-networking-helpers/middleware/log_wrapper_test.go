package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/cf-networking-helpers/middleware"
	"code.cloudfoundry.org/cf-networking-helpers/middleware/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var LogsWith = func(level lager.LogLevel, msg string) types.GomegaMatcher {
	return And(
		WithTransform(func(log lager.LogFormat) string {
			return log.Message
		}, Equal(msg)),
		WithTransform(func(log lager.LogFormat) lager.LogLevel {
			return log.LogLevel
		}, Equal(level)),
	)
}

var HaveLogData = func(nextMatcher types.GomegaMatcher) types.GomegaMatcher {
	return WithTransform(func(log lager.LogFormat) lager.Data {
		return log.Data
	}, nextMatcher)
}
var _ = Describe("LogWrap", func() {
	var (
		logWrapper        *middleware.LogWrapper
		logger            *lagertest.TestLogger
		loggableHandler   http.Handler
		wrappingHandler   http.Handler
		fakeUUIDGenerator *fakes.UUIDGenerator
		resp              *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-session")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		loggableHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v := r.Context().Value(middleware.LoggerKey); v != nil {
				if logger, ok := v.(lager.Logger); ok {
					logger = logger.Session("logger-group")
					logger.Info("written-in-loggable-handler")
				}
			}
		})

		fakeUUIDGenerator = &fakes.UUIDGenerator{}
		fakeUUIDGenerator.GenerateUUIDReturns("some-uuid", nil)
		logWrapper = &middleware.LogWrapper{
			UUIDGenerator: fakeUUIDGenerator,
		}

		wrappingHandler = logWrapper.LogWrap(logger, loggableHandler)
		resp = httptest.NewRecorder()
	})

	It("creates \"request-<UUID>\" session and passes it to loggableHandler", func() {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		Expect(err).NotTo(HaveOccurred())
		wrappingHandler.ServeHTTP(resp, req)

		Expect(resp.Header().Get("X-VCAP-Request-ID")).To(Equal("some-uuid"))

		Expect(logger.Logs()).To(HaveLen(3))
		Expect(logger.Logs()[0]).To(SatisfyAll(
			LogsWith(lager.DEBUG, "test-session.request_some-uuid.serving"),
			HaveLogData(SatisfyAll(
				HaveLen(4),
				HaveKeyWithValue("session", Equal("1")),
				HaveKeyWithValue("method", Equal("GET")),
				HaveKeyWithValue("request", Equal("http://example.com")),
				HaveKeyWithValue("request_guid", Equal("some-uuid")),
			)),
		))

		Expect(logger.Logs()[1]).To(SatisfyAll(
			LogsWith(lager.INFO, "test-session.request_some-uuid.logger-group.written-in-loggable-handler"),
			HaveLogData(SatisfyAll(
				HaveLen(4),
				HaveKeyWithValue("session", Equal("1.1")),
				HaveKeyWithValue("method", Equal("GET")),
				HaveKeyWithValue("request", Equal("http://example.com")),
				HaveKeyWithValue("request_guid", Equal("some-uuid")),
			)),
		))

		Expect(logger.Logs()[2]).To(SatisfyAll(
			LogsWith(lager.DEBUG, "test-session.request_some-uuid.done"),
			HaveLogData(SatisfyAll(
				HaveLen(4),
				HaveKeyWithValue("session", Equal("1")),
				HaveKeyWithValue("method", Equal("GET")),
				HaveKeyWithValue("request", Equal("http://example.com")),
				HaveKeyWithValue("request_guid", Equal("some-uuid")),
			)),
		))
	})

	Context("when uuid is provided on request header", func() {
		It("appends to the uuid on the request header", func() {
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("X-VCAP-Request-ID", `previous-uuid`)

			wrappingHandler.ServeHTTP(resp, req)

			Expect(resp.Header().Get("X-VCAP-Request-ID")).To(Equal("previous-uuid::some-uuid"))

			Expect(logger.Logs()).To(HaveLen(3))
			Expect(logger.Logs()[0]).To(SatisfyAll(
				LogsWith(lager.DEBUG, "test-session.request_previous-uuid::some-uuid.serving"),
				HaveLogData(SatisfyAll(
					HaveLen(4),
					HaveKeyWithValue("session", Equal("1")),
					HaveKeyWithValue("method", Equal("GET")),
					HaveKeyWithValue("request", Equal("http://example.com")),
					HaveKeyWithValue("request_guid", Equal("previous-uuid::some-uuid")),
				)),
			))

			Expect(logger.Logs()[1]).To(SatisfyAll(
				LogsWith(lager.INFO, "test-session.request_previous-uuid::some-uuid.logger-group.written-in-loggable-handler"),
				HaveLogData(SatisfyAll(
					HaveLen(4),
					HaveKeyWithValue("session", Equal("1.1")),
					HaveKeyWithValue("method", Equal("GET")),
					HaveKeyWithValue("request", Equal("http://example.com")),
					HaveKeyWithValue("request_guid", Equal("previous-uuid::some-uuid")),
				)),
			))

			Expect(logger.Logs()[2]).To(SatisfyAll(
				LogsWith(lager.DEBUG, "test-session.request_previous-uuid::some-uuid.done"),
				HaveLogData(SatisfyAll(
					HaveLen(4),
					HaveKeyWithValue("session", Equal("1")),
					HaveKeyWithValue("method", Equal("GET")),
					HaveKeyWithValue("request", Equal("http://example.com")),
					HaveKeyWithValue("request_guid", Equal("previous-uuid::some-uuid")),
				)),
			))
		})
	})

	Context("when creating the uuid fails", func() {
		BeforeEach(func() {
			fakeUUIDGenerator.GenerateUUIDReturns("", errors.New("ignored"))
		})
		It("ignores the error, creates \"request\" session and passes it to loggableHandler", func() {
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
			wrappingHandler.ServeHTTP(resp, req)

			Expect(logger.Logs()).To(HaveLen(3))
			Expect(logger.Logs()[0]).To(SatisfyAll(
				LogsWith(lager.DEBUG, "test-session.request.serving"),
				HaveLogData(SatisfyAll(
					HaveLen(3),
					HaveKeyWithValue("session", Equal("1")),
					HaveKeyWithValue("method", Equal("GET")),
					HaveKeyWithValue("request", Equal("http://example.com")),
				)),
			))

			Expect(logger.Logs()[1]).To(SatisfyAll(
				LogsWith(lager.INFO, "test-session.request.logger-group.written-in-loggable-handler"),
				HaveLogData(SatisfyAll(
					HaveLen(3),
					HaveKeyWithValue("session", Equal("1.1")),
					HaveKeyWithValue("method", Equal("GET")),
					HaveKeyWithValue("request", Equal("http://example.com")),
				)),
			))

			Expect(logger.Logs()[2]).To(SatisfyAll(
				LogsWith(lager.DEBUG, "test-session.request.done"),
				HaveLogData(SatisfyAll(
					HaveLen(3),
					HaveKeyWithValue("session", Equal("1")),
					HaveKeyWithValue("method", Equal("GET")),
					HaveKeyWithValue("request", Equal("http://example.com")),
				)),
			))
		})
	})
})
