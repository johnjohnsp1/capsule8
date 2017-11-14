package services

import (
	"net/http"

	// Include pprof
	_ "net/http/pprof"

	"golang.org/x/net/context"

	"github.com/golang/glog"
)

type ProfilingService struct {
	server *http.Server

	address string
}

func NewProfilingService(address string) *ProfilingService {
	return &ProfilingService{
		address: address,
	}
}

func (ps *ProfilingService) Name() string {
	return "Profiling HTTP endpoint"
}

func (ps *ProfilingService) Serve() error {
	glog.V(1).Infof("Serving profiling HTTP endpoints on %s",
		ps.address)

	ps.server = &http.Server{
		Addr: ps.address,
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
