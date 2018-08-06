package models_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/events/eventfakes"
	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/copilot/models/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("BackendSetRepo", func() {
	Describe("Get", func() {
		Context("when the ActualLRP is not running", func() {
			It("does not get added", func() {
				ticker := fakes.NewTicker()
				logger := lagertest.NewTestLogger("test")
				bbsEventer := &fakes.BBSEventer{}

				bs := models.NewBackendSetRepo(bbsEventer, logger, ticker.C)

				ef := &eventfakes.FakeEventSource{}
				bbsEventer.SubscribeToEventsReturns(ef, nil)

				firstLRP := &bbsmodels.ActualLRPGroup{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.ActualLRPKey{
							ProcessGuid: "other-guid",
						},
						State: bbsmodels.ActualLRPStateCrashed,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address: "11.11.11.11",
							Ports: []*bbsmodels.PortMapping{
								{HostPort: 2323, ContainerPort: 2424},
								{HostPort: 1111, ContainerPort: 2222},
							},
						},
					},
				}

				ef.NextReturns(bbsmodels.NewActualLRPCreatedEvent(firstLRP), nil)
				sig := make(<-chan os.Signal)
				ready := make(chan<- struct{})

				go bs.Run(sig, ready)

				ticker.C <- time.Time{}

				Eventually(func() []*api.Backend {
					return bs.Get("other-guid").GetBackends()
				}).Should(HaveLen(0))
			})
		})

		Context("when we miss a diego event", func() {
			It("runs a reconciliation to get all events", func() {
				ticker := fakes.NewTicker()
				logger := lagertest.NewTestLogger("test")
				bbsEventer := &fakes.BBSEventer{}

				bs := models.NewBackendSetRepo(bbsEventer, logger, ticker.C)

				ef := &eventfakes.FakeEventSource{}
				bbsEventer.SubscribeToEventsReturns(ef, nil)

				missedLRP := &bbsmodels.ActualLRPGroup{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.ActualLRPKey{
							ProcessGuid: "other-guid",
						},
						State: bbsmodels.ActualLRPStateRunning,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address: "11.11.11.11",
							Ports: []*bbsmodels.PortMapping{
								{HostPort: 2323, ContainerPort: 2424},
								{HostPort: 1111, ContainerPort: 2222},
							},
						},
					},
				}

				bbsEventer.ActualLRPGroupsReturns([]*bbsmodels.ActualLRPGroup{missedLRP}, nil)

				caughtLRPEvent := bbsmodels.NewActualLRPCreatedEvent(&bbsmodels.ActualLRPGroup{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.ActualLRPKey{
							ProcessGuid: "some-guid",
						},
						State: bbsmodels.ActualLRPStateRunning,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address: "10.10.10.10",
							Ports: []*bbsmodels.PortMapping{
								{HostPort: 1555, ContainerPort: 1000},
								{HostPort: 5685, ContainerPort: 2222},
							},
						},
					},
				})

				ef.NextReturns(caughtLRPEvent, nil)

				sig := make(<-chan os.Signal)
				ready := make(chan<- struct{})

				go bs.Run(sig, ready)

				ticker.C <- time.Time{}

				var backends []*api.Backend
				Eventually(func() *api.BackendSet {
					res := bs.Get("other-guid")
					backends = res.GetBackends()
					return res
				}).ShouldNot(BeNil())
				Expect(backends[0].Address).To(Equal("11.11.11.11"))
				Expect(backends[0].Port).To(Equal(uint32(2323)))

				Consistently(func() []*api.Backend {
					res := bs.Get("some-guid")
					return res.GetBackends()
				}).Should(HaveLen(1))
			})
		})

		Context("when successfully subscribed to diego events", func() {
			var (
				ticker     *fakes.Ticker
				logger     *lagertest.TestLogger
				bbsEventer *fakes.BBSEventer
				bs         *models.BackendSetRepo
				sig        chan os.Signal
				ready      chan<- struct{}
			)

			BeforeEach(func() {
				ticker = fakes.NewTicker()
				logger = lagertest.NewTestLogger("test")
				bbsEventer = &fakes.BBSEventer{}
				sig = make(chan os.Signal, 2)
				ready = make(chan<- struct{})

				bs = models.NewBackendSetRepo(bbsEventer, logger, ticker.C)
			})

			AfterEach(func() {
				sig <- os.Kill
			})

			It("returns a backendset", func() {
				ef := &eventfakes.FakeEventSource{}
				bbsEventer.SubscribeToEventsReturns(ef, nil)

				lrpEvent := bbsmodels.NewActualLRPCreatedEvent(&bbsmodels.ActualLRPGroup{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.ActualLRPKey{
							ProcessGuid: "meow",
						},
						State: bbsmodels.ActualLRPStateRunning,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address: "10.10.10.10",
							Ports: []*bbsmodels.PortMapping{
								{HostPort: 1555, ContainerPort: 1000},
								{HostPort: 5685, ContainerPort: 2222},
							},
						},
					},
				})

				ef.NextStub = func() (bbsmodels.Event, error) {
					switch ef.NextCallCount() {
					case 1:
						return lrpEvent, nil
					default:
						return nil, errors.New("whoops")
					}
				}

				go bs.Run(sig, ready)

				var backends []*api.Backend
				Eventually(func() *api.BackendSet {
					res := bs.Get("meow")
					backends = res.GetBackends()
					return res
				}).ShouldNot(BeNil())
				Expect(backends[0].Address).To(Equal("10.10.10.10"))
				Expect(backends[0].Port).To(Equal(uint32(1555)))
			})

			Context("when delete event is received", func() {
				It("removes backend from the repo", func() {
					ef := &eventfakes.FakeEventSource{}
					bbsEventer.SubscribeToEventsReturns(ef, nil)

					lrpEvent := bbsmodels.NewActualLRPCreatedEvent(&bbsmodels.ActualLRPGroup{
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.ActualLRPKey{
								ProcessGuid: "meow",
							},
							State: bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address: "10.10.10.10",
								Ports: []*bbsmodels.PortMapping{
									{HostPort: 1555, ContainerPort: 1000},
									{HostPort: 5685, ContainerPort: 2222},
								},
							},
						},
					})

					deletedLRPEvent := bbsmodels.NewActualLRPRemovedEvent(&bbsmodels.ActualLRPGroup{
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.ActualLRPKey{
								ProcessGuid: "meow",
							},
							State: bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address: "10.10.10.10",
								Ports: []*bbsmodels.PortMapping{
									{HostPort: 1555, ContainerPort: 1000},
									{HostPort: 5685, ContainerPort: 2222},
								},
							},
						},
					})

					wait := make(chan struct{})
					ef.NextStub = func() (bbsmodels.Event, error) {
						switch ef.NextCallCount() {
						case 1:
							return lrpEvent, nil
						case 2:
							<-wait
							return deletedLRPEvent, nil
						default:
							return nil, errors.New("whoops")
						}
					}

					go bs.Run(sig, ready)

					Eventually(func() []*api.Backend {
						res := bs.Get("meow")
						return res.GetBackends()
					}, "2s").Should(HaveLen(1))
					wait <- struct{}{}

					Eventually(func() []*api.Backend {
						res := bs.Get("meow")
						return res.GetBackends()
					}, "2s").Should(HaveLen(0))
				})
			})

			Context("when delete event is missed", func() {
				Context("when reconciliation runs", func() {
					It("removes backend from the repo", func() {
						ef := &eventfakes.FakeEventSource{}
						bbsEventer.SubscribeToEventsReturns(ef, nil)

						firstLRP := &bbsmodels.ActualLRPGroup{
							Instance: &bbsmodels.ActualLRP{
								ActualLRPKey: bbsmodels.ActualLRPKey{
									ProcessGuid: "other-guid",
								},
								State: bbsmodels.ActualLRPStateRunning,
								ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
									Address: "11.11.11.11",
									Ports: []*bbsmodels.PortMapping{
										{HostPort: 2323, ContainerPort: 2424},
									},
								},
							},
						}

						secondLRP := &bbsmodels.ActualLRPGroup{
							Instance: &bbsmodels.ActualLRP{
								ActualLRPKey: bbsmodels.ActualLRPKey{
									ProcessGuid: "any-guid",
								},
								State: bbsmodels.ActualLRPStateRunning,
								ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
									Address: "10.10.10.10",
									Ports: []*bbsmodels.PortMapping{
										{HostPort: 4545, ContainerPort: 4646},
									},
								},
							},
						}

						ef.NextReturns(bbsmodels.NewActualLRPCrashedEvent(&bbsmodels.ActualLRP{}, &bbsmodels.ActualLRP{}), nil)
						bbsEventer.ActualLRPGroupsReturnsOnCall(0, []*bbsmodels.ActualLRPGroup{firstLRP}, nil)
						bbsEventer.ActualLRPGroupsReturnsOnCall(1, []*bbsmodels.ActualLRPGroup{secondLRP}, nil)

						go bs.Run(sig, ready)
						ticker.C <- time.Time{}

						Eventually(func() []*api.Backend {
							return bs.Get("other-guid").GetBackends()
						}).Should(HaveLen(1))

						ticker.C <- time.Time{}

						Eventually(func() []*api.Backend {
							return bs.Get("other-guid").GetBackends()
						}).Should(HaveLen(0))

						Eventually(func() []*api.Backend {
							return bs.Get("any-guid").GetBackends()
						}).Should(HaveLen(1))
					})
				})
			})
		})
	})

	Context("when an error occurs", func() {
		Context("when the event stream fails", func() {
			It("logs an error", func() {
				ticker := fakes.NewTicker()
				logger := lagertest.NewTestLogger("test")
				bbsEventer := &fakes.BBSEventer{}

				bs := models.NewBackendSetRepo(bbsEventer, logger, ticker.C)

				ef := &eventfakes.FakeEventSource{}
				bbsEventer.SubscribeToEventsReturns(ef, nil)

				sig := make(<-chan os.Signal)
				ready := make(chan<- struct{})

				ef.NextReturns(nil, errors.New("stream error"))
				go bs.Run(sig, ready)

				Eventually(func() []lager.LogFormat {
					return logger.Logs()
				}).ShouldNot(HaveLen(0))

				Expect(logger.Logs()[0].Data["events-error"]).To(Equal("stream error"))
			})
		})

		Context("when getting all actual LRP groups", func() {
			Context("when reconciling the lrps fails", func() {
				It("logs an error", func() {
					ticker := fakes.NewTicker()
					logger := lagertest.NewTestLogger("test")
					bbsEventer := &fakes.BBSEventer{}
					bs := models.NewBackendSetRepo(bbsEventer, logger, ticker.C)

					ef := &eventfakes.FakeEventSource{}
					bbsEventer.SubscribeToEventsReturns(ef, nil)

					sig := make(<-chan os.Signal)
					ready := make(chan<- struct{})

					ef.NextReturns(bbsmodels.NewActualLRPCrashedEvent(&bbsmodels.ActualLRP{}, &bbsmodels.ActualLRP{}), nil)

					bbsEventer.ActualLRPGroupsReturns(nil, errors.New("lrp-groups-error"))

					go bs.Run(sig, ready)

					ticker.C <- time.Time{}

					Eventually(logger.Buffer).Should(gbytes.Say("lrp-groups-error"))
				})
			})
		})

		Context("when subscribing to events fails", func() {
			It("returns an error", func() {
				ticker := fakes.NewTicker()
				logger := lagertest.NewTestLogger("test")
				bbsEventer := &fakes.BBSEventer{}

				bs := models.NewBackendSetRepo(bbsEventer, logger, ticker.C)

				bbsEventer.SubscribeToEventsReturns(nil, errors.New("subscribe-error"))

				sig := make(<-chan os.Signal)
				ready := make(chan<- struct{})

				err := bs.Run(sig, ready)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("subscribe-error")))
			})
		})
	})
})
