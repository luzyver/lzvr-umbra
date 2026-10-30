package wg

import (
	"log/slog"
	"net"
	"sync"
)

// uapiServer memiliki listener dan koneksinya agar shutdown tidak menunggu klien diam.
type uapiServer struct {
	listener net.Listener
	done     chan struct{}
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	closing  bool
	handlers sync.WaitGroup
	once     sync.Once
	err      error
}

func startUAPI(l net.Listener, handle func(net.Conn)) *uapiServer {
	s := &uapiServer{listener: l, done: make(chan struct{}), conns: make(map[net.Conn]struct{})}
	go func() {
		defer close(s.done)
		for {
			c, err := l.Accept()
			if err != nil {
				s.mu.Lock()
				closing := s.closing
				s.mu.Unlock()
				if !closing {
					slog.Error("listener UAPI berhenti", "err", err)
				}
				return
			}
			s.mu.Lock()
			s.conns[c] = struct{}{}
			s.mu.Unlock()
			s.handlers.Add(1)
			go func() {
				defer s.handlers.Done()
				defer func() {
					_ = c.Close() // IpcHandle juga menutup koneksi saat selesai.
					s.mu.Lock()
					delete(s.conns, c)
					s.mu.Unlock()
				}()
				handle(c)
			}()
		}
	}()
	return s
}

func (s *uapiServer) Close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.closing = true
		s.mu.Unlock()
		s.err = s.listener.Close()
		<-s.done
		s.mu.Lock()
		for c := range s.conns {
			_ = c.Close()
		}
		s.mu.Unlock()
		s.handlers.Wait()
	})
	return s.err
}
