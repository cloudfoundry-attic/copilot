package models_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/events/eventfakes"
	bbsmodels "code.cloudfoundry.org/bbs/models"
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
				bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

				firstLRP := &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.ActualLRPKey{
						ProcessGuid: "other-guid",
					},
					State:    bbsmodels.ActualLRPStateCrashed,
					Presence: bbsmodels.ActualLRP_Ordinary,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "11.11.11.11",
						Ports: []*bbsmodels.PortMapping{
							{HostPort: 2323, ContainerPort: 2424},
							{HostPort: 1111, ContainerPort: 2222},
						},
					},
				}

				ef.NextReturns(bbsmodels.NewActualLRPInstanceCreatedEvent(firstLRP), nil)
				sig := make(<-chan os.Signal)
				ready := make(chan<- struct{})

				go bs.Run(sig, ready)

				ticker.C <- time.Time{}

				Eventually(func() []*models.Backend {
					return bs.Get("other-guid").Backends
				}).Should(HaveLen(0))
			})
		})

		Context("when the ActualLRP is evacuating", func() {
			It("does not get added", func() {
				ticker := fakes.NewTicker()
				logger := lagertest.NewTestLogger("test")
				bbsEventer := &fakes.BBSEventer{}

				bs := models.NewBackendSetRepo(bbsEventer, logger, ticker.C)

				ef := &eventfakes.FakeEventSource{}
				bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

				firstLRP := &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.ActualLRPKey{
						ProcessGuid: "other-guid",
					},
					State:    bbsmodels.ActualLRPStateRunning,
					Presence: bbsmodels.ActualLRP_Evacuating,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "11.11.11.11",
						Ports: []*bbsmodels.PortMapping{
							{HostPort: 2323, ContainerPort: 2424},
							{HostPort: 1111, ContainerPort: 2222},
						},
					},
				}

				ef.NextReturns(bbsmodels.NewActualLRPInstanceCreatedEvent(firstLRP), nil)
				sig := make(<-chan os.Signal)
				ready := make(chan<- struct{})

				go bs.Run(sig, ready)

				ticker.C <- time.Time{}

				Eventually(func() []*models.Backend {
					return bs.Get("other-guid").Backends
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
				bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

				missedLRP := &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.ActualLRPKey{
						ProcessGuid: "other-guid",
					},
					State:    bbsmodels.ActualLRPStateRunning,
					Presence: bbsmodels.ActualLRP_Ordinary,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "11.11.11.11",
						Ports: []*bbsmodels.PortMapping{
							{HostPort: 2323, ContainerPort: 2424},
							{HostPort: 1111, ContainerPort: 2222},
						},
					},
				}

				caughtLRP := &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.ActualLRPKey{
						ProcessGuid: "some-guid",
					},
					State:    bbsmodels.ActualLRPStateRunning,
					Presence: bbsmodels.ActualLRP_Ordinary,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.10.10.10",
						Ports: []*bbsmodels.PortMapping{
							{HostPort: 1555, ContainerPort: 1000},
							{HostPort: 5685, ContainerPort: 2222},
						},
					},
				}

				bbsEventer.ActualLRPsReturns([]*bbsmodels.ActualLRP{missedLRP, caughtLRP}, nil)

				caughtLRPEvent := bbsmodels.NewActualLRPInstanceCreatedEvent(caughtLRP)
				ef.NextReturns(caughtLRPEvent, nil)

				sig := make(<-chan os.Signal)
				ready := make(chan<- struct{})

				go bs.Run(sig, ready)

				ticker.C <- time.Time{}

				var backends []*models.Backend
				Eventually(func() []*models.Backend {
					res := bs.Get("other-guid")
					backends = res.Backends
					return backends
				}).ShouldNot(BeEmpty())
				Expect(backends[0].Address).To(Equal("11.11.11.11"))
				Expect(backends[0].Port).To(Equal(uint32(2323)))

				// Because the caughtLRP is always being emitted as an event there may be
				// duplicate entries (since it is also in the list of BBS LRP Groups), so we expect at least one
				Consistently(func() int {
					res := bs.Get("some-guid")
					return len(res.Backends)
				}).Should(BeNumerically(">=", 1))
			})
		})

		Context("when successfully subscribed to diego events", func() {
			var (
				ticker     *fakes.Ticker
				logger     *lagertest.TestLogger
				bbsEventer *fakes.BBSEventer
				bs         models.BackendSetRepo
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
				bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

				lrpEvent := bbsmodels.NewActualLRPInstanceCreatedEvent(&bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.ActualLRPKey{
						ProcessGuid: "meow",
					},
					State:    bbsmodels.ActualLRPStateRunning,
					Presence: bbsmodels.ActualLRP_Ordinary,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address:         "10.10.10.10",
						InstanceAddress: "13.13.13.13",
						Ports: []*bbsmodels.PortMapping{
							{HostPort: 1555, ContainerPort: 1000},
							{HostPort: 5685, ContainerPort: 2222},
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

				var backends []*models.Backend
				Eventually(func() []*models.Backend {
					res := bs.Get("meow")
					backends = res.Backends
					return backends
				}).ShouldNot(BeEmpty())
				Expect(backends[0].Address).To(Equal("10.10.10.10"))
				Expect(backends[0].Port).To(Equal(uint32(1555)))
				Expect(backends[0].ContainerPort).To(Equal(uint32(1000)))

				Eventually(func() []*models.Backend {
					res := bs.GetInternalBackends("meow")
					backends = res.Backends
					return backends
				}).ShouldNot(BeEmpty())
				Expect(backends[0].Address).To(Equal("13.13.13.13"))
				Expect(backends[0].Port).To(Equal(uint32(1000)))
				Expect(backends[0].ContainerPort).To(Equal(uint32(1000)))
			})

			Context("when a event is sent twice", func() {
				It("de-duplicates the backend", func() {
					ef := &eventfakes.FakeEventSource{}
					bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

					lrpEvent := bbsmodels.NewActualLRPInstanceCreatedEvent(&bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.ActualLRPKey{
							ProcessGuid: "meow",
						},
						State:    bbsmodels.ActualLRPStateRunning,
						Presence: bbsmodels.ActualLRP_Ordinary,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address:         "10.10.10.10",
							InstanceAddress: "13.13.13.13",
							Ports: []*bbsmodels.PortMapping{
								{HostPort: 1555, ContainerPort: 1000},
								{HostPort: 5685, ContainerPort: 2222},
							},
						},
					})

					ef.NextStub = func() (bbsmodels.Event, error) {
						switch ef.NextCallCount() {
						case 1:
							return lrpEvent, nil
						case 2:
							return lrpEvent, nil
						default:
							return nil, errors.New("whoops")
						}
					}

					go bs.Run(sig, ready)

					var backends []*models.Backend
					Eventually(func() []*models.Backend {
						res := bs.Get("meow")
						backends = res.Backends
						return backends
					}).Should(HaveLen(1))
					Expect(backends[0].Address).To(Equal("10.10.10.10"))
					Expect(backends[0].Port).To(Equal(uint32(1555)))
					Expect(backends[0].ContainerPort).To(Equal(uint32(1000)))

					Eventually(func() []*models.Backend {
						res := bs.GetInternalBackends("meow")
						backends = res.Backends
						return backends
					}).Should(HaveLen(1))
					Expect(backends[0].Address).To(Equal("13.13.13.13"))
					Expect(backends[0].Port).To(Equal(uint32(1000)))
					Expect(backends[0].ContainerPort).To(Equal(uint32(1000)))
				})
			})

			Context("when delete event is received", func() {
				It("removes backend from the repo", func() {
					ef := &eventfakes.FakeEventSource{}
					bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

					lrpEvent := bbsmodels.NewActualLRPInstanceCreatedEvent(&bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.ActualLRPKey{
							ProcessGuid: "meow",
						},
						State:    bbsmodels.ActualLRPStateRunning,
						Presence: bbsmodels.ActualLRP_Ordinary,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address: "10.10.10.10",
							Ports: []*bbsmodels.PortMapping{
								{HostPort: 1555, ContainerPort: 1000},
								{HostPort: 5685, ContainerPort: 2222},
							},
						},
					})

					deletedLRPEvent := bbsmodels.NewActualLRPInstanceRemovedEvent(&bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.ActualLRPKey{
							ProcessGuid: "meow",
						},
						State:    bbsmodels.ActualLRPStateRunning,
						Presence: bbsmodels.ActualLRP_Ordinary,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address: "10.10.10.10",
							Ports: []*bbsmodels.PortMapping{
								{HostPort: 1555, ContainerPort: 1000},
								{HostPort: 5685, ContainerPort: 2222},
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

					Eventually(func() []*models.Backend {
						res := bs.Get("meow")
						return res.Backends
					}, "2s").Should(HaveLen(1))
					wait <- struct{}{}

					Eventually(func() []*models.Backend {
						res := bs.Get("meow")
						return res.Backends
					}, "2s").Should(HaveLen(0))
				})
			})

			Context("when delete event is missed", func() {
				Context("when reconciliation runs", func() {
					It("removes backend from the repo", func() {
						ef := &eventfakes.FakeEventSource{}
						bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

						firstLRP := &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.ActualLRPKey{
								ProcessGuid: "other-guid",
							},
							State:    bbsmodels.ActualLRPStateRunning,
							Presence: bbsmodels.ActualLRP_Ordinary,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address: "11.11.11.11",
								Ports: []*bbsmodels.PortMapping{
									{HostPort: 2323, ContainerPort: 2424},
								},
							},
						}

						secondLRP := &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.ActualLRPKey{
								ProcessGuid: "any-guid",
							},
							State:    bbsmodels.ActualLRPStateRunning,
							Presence: bbsmodels.ActualLRP_Ordinary,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address: "10.10.10.10",
								Ports: []*bbsmodels.PortMapping{
									{HostPort: 4545, ContainerPort: 4646},
								},
							},
						}

						ef.NextReturns(bbsmodels.NewActualLRPCrashedEvent(&bbsmodels.ActualLRP{}, &bbsmodels.ActualLRP{}), nil)
						bbsEventer.ActualLRPsReturnsOnCall(0, []*bbsmodels.ActualLRP{firstLRP}, nil)
						bbsEventer.ActualLRPsReturnsOnCall(1, []*bbsmodels.ActualLRP{secondLRP}, nil)

						go bs.Run(sig, ready)
						ticker.C <- time.Time{}

						Eventually(func() []*models.Backend {
							return bs.Get("other-guid").Backends
						}).Should(HaveLen(1))

						ticker.C <- time.Time{}

						Eventually(func() []*models.Backend {
							return bs.Get("other-guid").Backends
						}).Should(HaveLen(0))

						Eventually(func() []*models.Backend {
							return bs.Get("any-guid").Backends
						}).Should(HaveLen(1))
					})
				})
			})
		})
	})

	Context("when there is no backend set for a GUID", func() {
		It("returns nil", func() {
			ticker := fakes.NewTicker()
			logger := lagertest.NewTestLogger("test")
			bbsEventer := &fakes.BBSEventer{}

			bsr := models.NewBackendSetRepo(bbsEventer, logger, ticker.C)

			set := bsr.Get("some-guid-does-not-exist")
			Expect(set.Backends).To(BeEmpty())

			set = bsr.GetInternalBackends("some-guid-not-here")
			Expect(set.Backends).To(BeEmpty())
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
				bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

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

		Context("when getting all actual LRPs", func() {
			Context("when reconciling the lrps fails", func() {
				It("logs an error", func() {
					ticker := fakes.NewTicker()
					logger := lagertest.NewTestLogger("test")
					bbsEventer := &fakes.BBSEventer{}
					bs := models.NewBackendSetRepo(bbsEventer, logger, ticker.C)

					ef := &eventfakes.FakeEventSource{}
					bbsEventer.SubscribeToInstanceEventsReturns(ef, nil)

					sig := make(<-chan os.Signal)
					ready := make(chan<- struct{})

					ef.NextReturns(bbsmodels.NewActualLRPCrashedEvent(&bbsmodels.ActualLRP{}, &bbsmodels.ActualLRP{}), nil)

					bbsEventer.ActualLRPsReturns(nil, errors.New("lrp-groups-error"))

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

				bbsEventer.SubscribeToInstanceEventsReturns(nil, errors.New("subscribe-error"))

				sig := make(<-chan os.Signal)
				ready := make(chan<- struct{})

				err := bs.Run(sig, ready)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("subscribe-error")))
			})
		})
	})
})
