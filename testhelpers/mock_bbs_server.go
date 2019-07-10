package testhelpers

import (
	"net/http"
	"strconv"

	"code.cloudfoundry.org/bbs/events"
	bbsmodels "code.cloudfoundry.org/bbs/models"
	"github.com/gogo/protobuf/proto"
	"github.com/onsi/gomega/ghttp"
)

func NewMockBBSServer() *MockBBSServer {
	bbsServer := ghttp.NewUnstartedServer()
	bbsServer.RouteToHandler("POST", "/v1/cells/list.r1", func(w http.ResponseWriter, req *http.Request) {
		cellsResponse := bbsmodels.CellsResponse{}
		data, _ := proto.Marshal(&cellsResponse)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	return &MockBBSServer{Server: bbsServer}
}

type MockBBSServer struct {
	Server *ghttp.Server
}

func (m *MockBBSServer) SetPostV1EventsResponse(actualLRP *bbsmodels.ActualLRP) {
	lrpEvent := bbsmodels.NewActualLRPInstanceCreatedEvent(actualLRP)
	m.Server.RouteToHandler("POST", "/v1/events/lrp_instances.r1", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Add("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "identity")
		w.WriteHeader(http.StatusOK)

		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}

		defer func() {
			conn.Close()
		}()

		rw.Flush()

		event, _ := events.NewEventFromModelEvent(0, lrpEvent)
		event.Write(conn)
	})
}

func (m *MockBBSServer) SetPostV1ActualLRPsList(actualLRPs []*bbsmodels.ActualLRP) {
	m.Server.RouteToHandler("POST", "/v1/actual_lrps/list", func(w http.ResponseWriter, req *http.Request) {
		actualLRPResponse := bbsmodels.ActualLRPsResponse{
			ActualLrps: actualLRPs,
		}
		data, _ := proto.Marshal(&actualLRPResponse)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}
