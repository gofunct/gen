package proxy

import (
	"github.com/gofunct/grpcgen/errors"
	"github.com/prometheus/common/log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/handlers"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Proxy struct {
	Mux       *http.ServeMux
	Gateway   *runtime.ServeMux
	Formatter handlers.LogFormatter
	DialOpts  []grpc.DialOption
	Config 		*viper.Viper
}

func NewProxy(vi *viper.Viper, service string) *Proxy {
	err := NewProxyViperizer(vi, service)
	errors.IfErr("failed to creage proxy config", err)
	formatter := LogHandlers()
	mux := NewMux()

	gwmux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(incomingHeaderMatcher),
		runtime.WithOutgoingHeaderMatcher(outgoingHeaderMatcher),
	)
	mux.Handle("/", handlers.CustomLoggingHandler(os.Stdout, gwmux, formatter))
	log.Info("gateway handler registered-->", "/")
	logrus.Infof("Proxying requests to gRPC service at '%s'", vi.GetString("proxy.backend"))

	opts := NewDialOpts()



	return &Proxy{
		Mux:       mux,
		Gateway:   gwmux,
		Formatter: formatter,
		DialOpts:  opts,
		Config:    vi,
	}
}

// SignalRunner runs a runner function until an interrupt signal is received, at which point it
// will call stopper.
func (p *Proxy) Shutdown(runner, stopper func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	go func() {
		runner()
	}()

	logrus.Info("hit Ctrl-C to shutdown")
	select {
	case <-signals:
		stopper()
	}
}

func (p *Proxy) Listen(ctx context.Context) {


	server := &http.Server{Addr: ":8080", Handler: p.Mux}

	p.Shutdown(
		func() {
			logrus.Infof("launching http server on %v", server.Addr)
			if err := server.ListenAndServe(); err != nil {
				logrus.Fatalf("Could not start http server: %v", err)
			}
		},
		func() {
			shutdown, _ := context.WithTimeout(ctx, 10*time.Second)
			server.Shutdown(shutdown)
		})
}


