package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	//"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/negroni"
	"github.com/zbindenren/negroni-prometheus"
	"gopkg.in/unrolled/secure.v1"

	"github.com/ph0m1/porta/config/viper"
	"github.com/ph0m1/porta/logging/gologging"
	"github.com/ph0m1/porta/proxy"
	"github.com/ph0m1/porta/router/mux"
)

func main() {
	port := flag.Int("p", 0, "Port of the service")
	logLevel := flag.String("l", "ERROR", "Logging level")
	debug := flag.Bool("d", true, "Enable the debug")
	configFile := flag.String("c", "../etc/configuration.json", "Path to configuration filename")
	flag.Parse()

	parser := viper.New()
	serviceConfig, err := parser.Parse(*configFile)
	if err != nil {
		log.Fatal("ERROR:", err.Error())
	}

	serviceConfig.Debug = serviceConfig.Debug || *debug
	if *port != 0 {
		serviceConfig.Port = *port
	}
	logger, err := gologging.NewLogger(*logLevel, os.Stdout, "[O.o]")
	if err != nil {
		log.Fatal("ERROR:", err.Error())
	}
	secureMiddleware := secure.New(secure.Options{
		AllowedHosts:          []string{"127.0.0.1:8080", "example.com", "ssl.example.com"},
		SSLRedirect:           false,
		SSLHost:               "ssl.example.com",
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
		STSSeconds:            315360000,
		STSIncludeSubdomains:  true,
		STSPreload:            true,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "default-src 'self'",
	})

	routerFactory := mux.NewFactory(mux.Config{
		Engine:         newNegroniEngine(),
		ProxyFactory:   proxy.DefaultFactory(logger),
		Middlewares:    []mux.HandlerMiddleware{secureMiddleware},
		Logger:         logger,
		HandlerFactory: mux.EndpointHandler,
	})
	routerFactory.New().Run(serviceConfig)

}

func newNegroniEngine() negroniEngine {
	muxEngine := mux.DefaultEngine()
	negroniRouter := negroni.Classic()
	negroniRouter.UseHandler(muxEngine)

	m := negroniprometheus.NewMiddleware("serviceName")
	muxEngine.Handle("/__metrics", promhttp.Handler())
	negroniRouter.Use(m)
	return negroniEngine{muxEngine, negroniRouter}
}

type negroniEngine struct {
	r mux.Engine
	n *negroni.Negroni
}

// Handle implements the mux.Engine interface from the router package
func (e negroniEngine) Handle(pattern string, handler http.Handler) {
	e.r.Handle(pattern, handler)
}

// ServeHTTP implements the http.Handler interface from the stdlib: net/http
func (e negroniEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.n.ServeHTTP(w, r)
}
