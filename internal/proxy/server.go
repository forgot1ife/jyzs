package proxy

import (
	"bytes"
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"jyzs_proxy_poc/internal/processor"
	"jyzs_proxy_poc/internal/rules"

	"github.com/elazarl/goproxy"
)

type Config struct {
	ListenAddr   string
	MaxBodyBytes int64
	EnableMITM   bool
	ExportCAPath string
	RuleSet      *rules.RuleSet
	Processor    *processor.Processor
}

type Server struct {
	httpServer *http.Server
}

type reqMeta struct {
	requestBody []byte
	requestSize int
	matched     []rules.Rule
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 512 * 1024
	}
	if cfg.ExportCAPath != "" {
		if err := exportCACert(cfg.ExportCAPath); err != nil {
			return nil, err
		}
	}

	p := goproxy.NewProxyHttpServer()
	p.Verbose = false

	if cfg.EnableMITM {
		p.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	} else {
		p.OnRequest().HandleConnectFunc(func(host string, _ *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			return goproxy.OkConnect, host
		})
	}

	p.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		body, bodySize := cloneBody(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
		matched := cfg.RuleSet.MatchRules(req)
		ctx.UserData = reqMeta{
			requestBody: clipped(body, cfg.MaxBodyBytes),
			requestSize: bodySize,
			matched:     matched,
		}
		return req, nil
	})

	p.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if resp == nil || resp.Request == nil {
			return resp
		}
		meta, _ := ctx.UserData.(reqMeta)
		if len(meta.matched) == 0 {
			return resp
		}

		responseBody, responseSize := cloneBody(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(responseBody))
		if !isInspectableContent(resp.Header.Get("Content-Type")) {
			return resp
		}
		responseBody = clipped(responseBody, cfg.MaxBodyBytes)

		for _, rule := range meta.matched {
			fields := rule.Extract(meta.requestBody, responseBody)
			if len(fields) == 0 {
				continue
			}
			_ = cfg.Processor.Handle(processor.EventInput{
				RuleName:     rule.Name,
				RecordType:   rule.RecordType,
				Method:       resp.Request.Method,
				Host:         resp.Request.URL.Host,
				Path:         resp.Request.URL.Path,
				StatusCode:   resp.StatusCode,
				RequestSize:  meta.requestSize,
				ResponseSize: responseSize,
				Fields:       fields,
			})
		}
		return resp
	})

	httpSrv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: p,
	}
	return &Server{httpServer: httpSrv}, nil
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func cloneBody(body io.ReadCloser) ([]byte, int) {
	if body == nil {
		return nil, 0
	}
	defer body.Close()
	b, _ := io.ReadAll(body)
	return b, len(b)
}

func clipped(body []byte, limit int64) []byte {
	if limit <= 0 || int64(len(body)) <= limit {
		return body
	}
	return body[:limit]
}

func isInspectableContent(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "text/") ||
		strings.Contains(ct, "application/javascript") ||
		strings.Contains(ct, "application/xml") ||
		strings.Contains(ct, "application/x-www-form-urlencoded")
}

func exportCACert(path string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(goproxy.GoproxyCa.Certificate) == 0 {
		return nil
	}
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: goproxy.GoproxyCa.Certificate[0],
	}
	return pem.Encode(f, block)
}
