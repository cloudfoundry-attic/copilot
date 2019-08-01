package db_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/cf-networking-helpers/db"
	"code.cloudfoundry.org/cf-networking-helpers/fakes"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("RetriableConnector", func() {
	var (
		logger             *lagertest.TestLogger
		sleeper            *fakes.Sleeper
		retriableConnector *db.RetriableConnector
		numTries           int
		passedContext      context.Context
	)

	BeforeEach(func() {
		sleeper = &fakes.Sleeper{}
		logger = lagertest.NewTestLogger("test")
		numTries = 0

		retriableConnector = &db.RetriableConnector{
			Logger:        logger,
			Sleeper:       sleeper,
			RetryInterval: time.Minute,
			Connector: func(config db.Config, context context.Context) (*db.ConnWrapper, error) {
				passedContext = context
				numTries++
				if numTries > 3 {
					return nil, nil
				}
				return nil, db.RetriableError{Inner: errors.New("welp")}
			},
		}
	})

	Context("when the inner Connector returns a non-retriable error", func() {
		It("returns the error immediately", func() {
			retriableConnector := db.RetriableConnector{
				Connector: func(db.Config, context.Context) (*db.ConnWrapper, error) {
					return nil, errors.New("banana")
				},
			}

			_, err := retriableConnector.GetConnectionPool(db.Config{}, context.Background())
			Expect(err).To(MatchError("banana"))
		})
	})

	Context("when the inner Connector returns a retriable error", func() {
		It("retries the connection", func() {
			retriableConnector.MaxRetries = 5

			ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
			_, err := retriableConnector.GetConnectionPool(db.Config{}, ctx)

			Expect(numTries).To(Equal(4))
			Expect(passedContext).To(Equal(ctx))
			Expect(err).NotTo(HaveOccurred())

		})

		It("waits between retries", func() {
			retriableConnector.MaxRetries = 5

			_, err := retriableConnector.GetConnectionPool(db.Config{}, context.Background())
			Expect(err).To(Succeed())

			Expect(sleeper.SleepCallCount()).To(Equal(3))
			Expect(sleeper.SleepArgsForCall(0)).To(Equal(time.Minute))
			Expect(sleeper.SleepArgsForCall(1)).To(Equal(time.Minute))
			Expect(sleeper.SleepArgsForCall(2)).To(Equal(time.Minute))

			Expect(logger).To(gbytes.Say("retrying due to getting an error"))
		})

		Context("when max retries have occurred", func() {
			It("stops retrying and returns the last error", func() {
				retriableConnector.MaxRetries = 10
				retriableConnector.Connector = func(db.Config, context.Context) (*db.ConnWrapper, error) {
					numTries++
					return nil, db.RetriableError{Inner: errors.New("welp")}
				}

				_, err := retriableConnector.GetConnectionPool(db.Config{}, context.Background())
				Expect(err).To(MatchError(db.RetriableError{Inner: errors.New("welp")}))

				Eventually(numTries).Should(Equal(10))
				Expect(sleeper.SleepCallCount()).To(Equal(9))
			})
		})
	})
})
