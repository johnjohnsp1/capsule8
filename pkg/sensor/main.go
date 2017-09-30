package sensor

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"net/http"

	// Include pprof
	_ "net/http/pprof"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/version"
	"github.com/golang/glog"
)

type profilingServer struct {
	server *http.Server
}

func (p *profilingServer) Name() string {
	return "Profiling HTTP endpoint"
}

func (p *profilingServer) Serve() error {
	glog.Infof("Serving profiling HTTP endpoints on 127.0.0.1:%d",
		config.Global.ProfilingPort)

	addr := fmt.Sprintf("127.0.0.1:%d", config.Global.ProfilingPort)

	p.server = &http.Server{
		Addr: addr,
	}

	serveErr := p.server.ListenAndServe()

	glog.V(1).Info(serveErr)
	return serveErr
}

func (p *profilingServer) Stop() {
	p.server.Shutdown(context.Background())
}

// Main is the main entrypoint for the sensor
func Main() {
	if len(config.Sensor.ListenAddr) > 0 {
		s := &gRPCServer{}
		Sensor.RegisterServer(s)
	}

	if config.Global.ProfilingPort > 0 {
		s := &profilingServer{}
		Sensor.RegisterServer(s)
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		glog.Infof("Caught signal %v, stopping Sensor", sig)
		s.Stop()
	}()

	err = s.Serve()
	if err != nil {
		glog.Error(err)
	}

	glog.Info("Exiting...")
}
