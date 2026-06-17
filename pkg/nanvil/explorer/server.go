package explorer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Server serves the nanvil block explorer UI and proxies JSON-RPC.
type Server struct {
	httpServer *http.Server
	rpcURL     string
	log        *zap.Logger
	addr       string
}

// New creates an explorer server bound to host:port, proxying RPC to rpcAddr.
func New(rpcAddr, host string, port int, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	return &Server{
		rpcURL: "http://" + rpcAddr,
		log:    log.With(zap.String("service", "explorer")),
		addr:   addr,
	}
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.addr
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// Start listens and serves the explorer.
func (s *Server) Start() error {
	target, err := url.Parse(s.rpcURL)
	if err != nil {
		return fmt.Errorf("parse rpc url: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		ResponseHeaderTimeout: 60 * time.Second,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		s.log.Warn("rpc proxy error", zap.Error(err))
		http.Error(w, "RPC unavailable", http.StatusBadGateway)
	}
	origDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		origDirector(r)
		r.Host = target.Host
	}

	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	fileServer := http.FileServer(http.FS(static))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		proxy.ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/ws", s.handleWSProxy)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && !strings.Contains(r.URL.Path, ".") {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       90 * time.Second,
		WriteTimeout:      90 * time.Second,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	s.addr = ln.Addr().String()

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("explorer stopped", zap.Error(err))
		}
	}()
	s.log.Info("explorer started", zap.String("url", "http://"+s.addr))
	return nil
}

func (s *Server) handleWSProxy(w http.ResponseWriter, r *http.Request) {
	clientConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("ws upgrade failed", zap.Error(err))
		return
	}
	defer clientConn.Close()

	wsURL := strings.Replace(s.rpcURL, "http://", "ws://", 1)
	if strings.HasPrefix(s.rpcURL, "https://") {
		wsURL = strings.Replace(s.rpcURL, "https://", "wss://", 1)
	}
	wsURL = strings.TrimSuffix(wsURL, "/") + "/ws"

	serverConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		s.log.Warn("ws dial failed", zap.String("url", wsURL), zap.Error(err))
		_ = clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "rpc ws unavailable"))
		return
	}
	defer serverConn.Close()

	errCh := make(chan error, 2)
	relay := func(from, to *websocket.Conn) {
		for {
			mt, msg, err := from.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := to.WriteMessage(mt, msg); err != nil {
				errCh <- err
				return
			}
		}
	}
	go relay(clientConn, serverConn)
	go relay(serverConn, clientConn)
	<-errCh
}

// Shutdown stops the explorer HTTP server.
func (s *Server) Shutdown() {
	if s.httpServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.log.Warn("explorer shutdown", zap.Error(err))
	}
}

// Ping checks whether the RPC endpoint is reachable.
func (s *Server) Ping() error {
	resp, err := http.Post(s.rpcURL, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"getblockcount","params":[]}`))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rpc status %d", resp.StatusCode)
	}
	return nil
}
