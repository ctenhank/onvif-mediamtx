package core

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/defs"
	"github.com/bluenviron/mediamtx/internal/logger"
	rtspsource "github.com/bluenviron/mediamtx/internal/source"
)

const (
	staticSourceHandlerRetryPause = 5 * time.Second
)

func resolveSource(s string, matches []string, query string, username string, password string) (string, error) {
	if len(matches) > 1 {
		for i, ma := range matches[1:] {
			s = strings.ReplaceAll(s, "$G"+strconv.FormatInt(int64(i+1), 10), ma)
		}
	}

	s = strings.ReplaceAll(s, "$MTX_QUERY", query)

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}

	u.User = url.UserPassword(username, password)

	return u.String(), nil
}

type staticSourceHandlerParent interface {
	logger.Writer
	staticSourceHandlerSetReady(context.Context, defs.PathSourceStaticSetReadyReq)
	staticSourceHandlerSetNotReady(context.Context, defs.PathSourceStaticSetNotReadyReq)
}

// staticSourceHandler is a static source handler.
type staticSourceHandler struct {
	conf           *conf.Path
	logLevel       conf.LogLevel
	readTimeout    conf.StringDuration
	writeTimeout   conf.StringDuration
	writeQueueSize int
	matches        []string
	parent         staticSourceHandlerParent

	ctx       context.Context
	ctxCancel func()
	instance  defs.StaticSource
	running   bool
	query     string

	// in
	chReloadConf          chan *conf.Path
	chInstanceSetReady    chan defs.PathSourceStaticSetReadyReq
	chInstanceSetNotReady chan defs.PathSourceStaticSetNotReadyReq

	// out
	done chan struct{}
}

func (s *staticSourceHandler) initialize() {
	s.chReloadConf = make(chan *conf.Path)
	s.chInstanceSetReady = make(chan defs.PathSourceStaticSetReadyReq)
	s.chInstanceSetNotReady = make(chan defs.PathSourceStaticSetNotReadyReq)

	switch {
	case strings.HasPrefix(s.conf.Source, "rtsp://") ||
		strings.HasPrefix(s.conf.Source, "rtsps://"):
		s.instance = &rtspsource.Source{
			ReadTimeout:    s.readTimeout,
			WriteTimeout:   s.writeTimeout,
			WriteQueueSize: s.writeQueueSize,
			Parent:         s,
		}
	default:
		panic("should not happen")
	}
}

func (s *staticSourceHandler) close(reason string) {
	s.stop(reason)
}

func (s *staticSourceHandler) start(onDemand bool, query string) {
	if s.running {
		panic("should not happen")
	}

	s.running = true
	s.query = query
	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	s.done = make(chan struct{})

	s.instance.Log(logger.Info, "started%s",
		func() string {
			if onDemand {
				return " on demand"
			}
			return ""
		}())

	go s.run()
}

func (s *staticSourceHandler) stop(reason string) {
	if !s.running {
		panic("should not happen")
	}

	s.running = false

	s.instance.Log(logger.Info, "stopped: %s", reason)

	s.ctxCancel()

	// we must wait since s.ctx is not thread safe
	<-s.done
}

// Log implements logger.Writer.
func (s *staticSourceHandler) Log(level logger.Level, format string, args ...interface{}) {
	s.parent.Log(level, format, args...)
}

func (s *staticSourceHandler) run() {
	defer close(s.done)

	var runCtx context.Context
	var runCtxCancel func()
	runErr := make(chan error)
	runReloadConf := make(chan *conf.Path)

	recreate := func() {
		resolvedSource, err := resolveSource(s.conf.Source, s.matches, s.query, s.conf.Username, s.conf.Password)
		if err != nil {
			s.Log(logger.Error, "Failed to resolve Source %s: %s", s.conf.Source, err)
			return
		}

		runCtx, runCtxCancel = context.WithCancel(context.Background())
		go func() {
			runErr <- s.instance.Run(defs.StaticSourceRunParams{
				Context:        runCtx,
				ResolvedSource: resolvedSource,
				Conf:           s.conf,
				ReloadConf:     runReloadConf,
			})
		}()
	}

	recreate()

	recreating := false
	recreateTimer := emptyTimer()

	for {
		select {
		case err := <-runErr:
			runCtxCancel()
			s.instance.Log(logger.Error, err.Error())
			recreating = true
			recreateTimer = time.NewTimer(staticSourceHandlerRetryPause)

		case req := <-s.chInstanceSetReady:
			s.parent.staticSourceHandlerSetReady(s.ctx, req)

		case req := <-s.chInstanceSetNotReady:
			s.parent.staticSourceHandlerSetNotReady(s.ctx, req)

		case newConf := <-s.chReloadConf:
			s.conf = newConf
			if !recreating {
				cReloadConf := runReloadConf
				cInnerCtx := runCtx
				go func() {
					select {
					case cReloadConf <- newConf:
					case <-cInnerCtx.Done():
					}
				}()
			}

		case <-recreateTimer.C:
			recreate()
			recreating = false

		case <-s.ctx.Done():
			if !recreating {
				runCtxCancel()
				<-runErr
			}
			return
		}
	}
}

func (s *staticSourceHandler) reloadConf(newConf *conf.Path) {
	ctx := s.ctx

	if !s.running {
		return
	}

	go func() {
		select {
		case s.chReloadConf <- newConf:
		case <-ctx.Done():
		}
	}()
}

// APISourceDescribe instanceements source.
func (s *staticSourceHandler) APISourceDescribe() defs.APIPathSourceOrReader {
	return s.instance.APISourceDescribe()
}

// setReady is called by a staticSource.
func (s *staticSourceHandler) SetReady(req defs.PathSourceStaticSetReadyReq) defs.PathSourceStaticSetReadyRes {
	req.Res = make(chan defs.PathSourceStaticSetReadyRes)
	select {
	case s.chInstanceSetReady <- req:
		res := <-req.Res

		if res.Err == nil {
			s.instance.Log(logger.Info, "ready: %s", defs.MediasInfo(req.Desc.Medias))
		}

		return res

	case <-s.ctx.Done():
		return defs.PathSourceStaticSetReadyRes{Err: fmt.Errorf("terminated")}
	}
}

// setNotReady is called by a staticSource.
func (s *staticSourceHandler) SetNotReady(req defs.PathSourceStaticSetNotReadyReq) {
	req.Res = make(chan struct{})
	select {
	case s.chInstanceSetNotReady <- req:
		<-req.Res
	case <-s.ctx.Done():
	}
}
