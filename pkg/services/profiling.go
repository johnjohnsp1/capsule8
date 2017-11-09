package services

import (
	"fmt"

	"net/http"

	// Include pprof
	_ "net/http/pprof"

	"golang.org/x/net/context"

	"github.com/golang/glog"
)

type ProfilingService struct {
	server *http.Server

	address string
	port    int
}

func NewProfilingService(address string, port int) *ProfilingService {
	return &ProfilingService{
		address: address,
		port:    port,
	}
}

func (ps *ProfilingService) Name() string {
	return "Profiling HTTP endpoint"
}

func (ps *ProfilingService) Serve() error {
	glog.V(1).Infof("Serving profiling HTTP endpoints on %s:%d",
		ps.address, ps.port)

	ps.server = &http.Server{
		Addr: fmt.Sprintf("%s:%d", ps.address, ps.port),
	}

	err := ps.server.ListenAndServe()
	if err != nil {
		glog.Errorf("Profiling HTTP error: %s", err)
	}

	return err
}

func (ps *ProfilingService) Stop() {
	ps.server.Shutdown(context.Background())
}
