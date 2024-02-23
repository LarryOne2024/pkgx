package apigateway

import (
	"context"
	"fmt"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/madlabx/pkgx/log"
	"github.com/madlabx/pkgx/lumberjackx"
	"github.com/madlabx/pkgx/viperx"
	"github.com/sirupsen/logrus"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

type LogConfig struct {
	Output  string
	Level   string
	Size    int
	BackNum int
	AgeDays int
}

type ApiGateway struct {
	Echo   *echo.Echo
	Logger *logrus.Logger
}

func New(ctx context.Context, logConfig LogConfig) (*ApiGateway, error) {
	agw := &ApiGateway{
		Echo: echo.New(),
	}
	if err := agw.initAccessLog(ctx, logConfig); err != nil {
		return nil, err
	}

	configEcho(agw.Echo)
	return agw, nil
}

func (agw *ApiGateway) Run(ip, port string) error {
	showEcho(agw.Echo)
	return startEcho(agw.Echo, fmt.Sprintf("%s:%s", ip, port))
}

func (agw *ApiGateway) Stop() {
	shutdownEcho(agw.Echo)
}

func (agw *ApiGateway) initAccessLog(ctx context.Context, lc LogConfig) error {
	if agw.Logger == nil {
		agw.Logger = log.New()
	}

	level, err := logrus.ParseLevel(lc.Level)
	if err != nil {
		return err
	}

	switch lc.Output {
	case "stdout":
		agw.Logger.SetOutput(os.Stdout)
	case "stderr":
		agw.Logger.SetOutput(os.Stderr)
	case "":
		agw.Logger.SetOutput(os.Stdout)
	default:
		agw.Logger.SetOutput(&lumberjackx.Logger{
			Ctx:        ctx,
			Filename:   lc.Output,
			MaxSize:    lc.Size,    // megabytes
			MaxBackups: lc.BackNum, //file number
			MaxAge:     lc.AgeDays, //days
			Compress:   true,       // disabled by default
			LocalTime:  true,
		})
	}

	agw.Logger.SetLevel(level)

	agw.Logger.SetFormatter(&log.TextFormatter{QuoteEmptyFields: true})

	return nil
}

func isPrintableTextContent(contentType string) bool {
	if strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "html") {
		return true
	}

	return false
}

func configEcho(e *echo.Echo) {
	// Tags to construct the Logger format.
	//
	// - time_unix
	// - time_unix_nano
	// - time_rfc3339
	// - time_rfc3339_nano
	// - time_custom
	// - id (Request ID)
	// - remote_ip
	// - uri
	// - host
	// - method
	// - path
	// - protocol
	// - referer
	// - user_agent
	// - status
	// - error
	// - latency (In nanoseconds)
	// - latency_human (Human readable)
	// - bytes_in (Bytes received)
	// - bytes_out (Bytes sent)
	// - header:<NAME>
	// - query:<NAME>
	// - form:<NAME>
	format := "${time_rfc3339} ${status} ${method} ${latency_human} ${host} ${remote_ip} ${bytes_in} ${bytes_out} ${uri} ${id} ${error}\n"
	e.Use(middleware.BodyDumpWithConfig(middleware.BodyDumpConfig{
		Handler: func(c echo.Context, reqBody []byte, resBody []byte) {
			lq := int(math.Min(float64(len(reqBody)), 2000))
			lp := int(math.Min(float64(len(resBody)), 2000))

			contentType := c.Response().Header().Get(echo.HeaderContentType)

			if isPrintableTextContent(contentType) || len(resBody) == 0 {
				log.Infof("%v, reqBody[%v]:{%v}, resBody[%v]:{%v}", c.Request().URL.String(), len(reqBody), string(reqBody[:lq]), len(resBody), string(resBody[:lp]))
			} else {
				log.Infof("%v, reqBody[%v]:{%v}, resBody[%v]:[Non-printable ContentType:%v]", c.Request().URL.String(), len(reqBody), string(reqBody[:lq]), len(resBody), contentType)
			}

			//accessLogger.Infof("%v, reqBody[%v]:{%v}, resBody[%v]:{%v}", c.Request().URL.String(), len(reqBody), string(reqBody[:lq]), len(resBody), string(resBody[:lp]))
		},
	}))
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: viperx.GetString("sys.accessFormat", format),
		//Output: accessLogger.Out,
		Output: log.StandardLogger().Out,
	}))

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		ExposeHeaders:    []string{"*"},
		AllowMethods:     []string{"*"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
		//AllowMethods: []string{Echo.GET, Echo.PUT, Echo.POST, Echo.DELETE},
	}))

	//TODO 检查是否可以恢复。不注释回无法下载css
	//e.Use(func(next Echo.HandlerFunc) Echo.HandlerFunc {
	//	return func(c Echo.Context) error {
	//		c.Response().Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)
	//		return next(c)
	//	}
	//})
}

func startEcho(e *echo.Echo, addr string) error {
	err := e.Start(addr)
	if err != nil {
		log.Errorf("Failed to bind address: %s, err[%v]", addr, err)
		return err
	}
	log.Infof("Start service listen on: %s", addr)
	return nil
}

func shutdownEcho(e *echo.Echo) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := e.Shutdown(ctx)
	if err != nil {
		log.Errorf("Failed to close Echo: %v", e)
	}
	log.Infof("Close service: %v", e)
}

func showEcho(e *echo.Echo) {

	routes := make([]struct {
		m string
		p string
	}, len(e.Routes()))
	for i, r := range e.Routes() {
		routes[i].m = r.Method
		routes[i].p = r.Path
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].p < routes[j].p })

	for _, r := range routes {
		log.Infof("%s %s", r.m, r.p)
	}
}
