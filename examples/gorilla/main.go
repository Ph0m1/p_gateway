package main

import (
	"flag"
	"log"
	"os"

	"github.com/ph0m1/porta/router/mux"
	"gopkg.in/unrolled/secure.v1"

	"github.com/ph0m1/porta/config"
	"github.com/ph0m1/porta/config/viper"
	"github.com/ph0m1/porta/logging"
	"github.com/ph0m1/porta/logging/gologging"
	"github.com/ph0m1/porta/proxy"
	"github.com/ph0m1/porta/router/gorilla"
)

func main() {
	port := flag.Int("p", 0, "Port of the service")
	logLevel := flag.String("l", "ERROR", "Enable the debug")
	debug := flag.Bool("d", false, "Enable the debug")
	configFile := flag.String("c", "../etc/configuration.json", "Path to configuration")
	flag.Parse()

	parser := viper.New()
	config.RoutingPattern = config.BracketsRouterPatternBuilder
	serviceConfig, err := parser.Parse(*configFile)
	if err != nil {
		log.Fatal("ERROR:", err.Error())
	}
	serviceConfig.Debug = serviceConfig.Debug || *debug
	if *port != 0 {
		serviceConfig.Port = *port
	}

	logger, err := gologging.NewLogger(*logLevel, os.Stdout, "[PORTA]")
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

	cfg := gorilla.DefaultConfig(customProxyFactory{logger: logger, factory: proxy.DefaultFactory(logger)}, logger)
	cfg.Middlewares = append(cfg.Middlewares, secureMiddleware)
	routerFactory := mux.NewFactory(cfg)
	routerFactory.New().Run(serviceConfig)
}

type customProxyFactory struct {
	logger  logging.Logger
	factory proxy.Factory
}

// New implements the Factory interface
func (cf customProxyFactory) New(cfg *config.EndpointConfig) (p proxy.Proxy, err error) {
	p, err = cf.factory.New(cfg)
	if err != nil {
		p = proxy.NewLoggingMiddleware(cf.logger, cfg.Endpoint)(p)
	}
	return
}
