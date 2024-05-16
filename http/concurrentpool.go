package services

import (
	"bytes"
	"context"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"sync"
	"time"
)

type ResponseCallback func(*http.Response)
type Callback func()
type SetHeaders func(*http.Request)

type HttpRequest struct {
	Method   string
	Url      string
	Payload  *string
	PoolNum  int // number of pool where it requested
	CallBack ResponseCallback
}

// ConcurrentPool allows execute a few http requests in parallel, in case all pools is busy request goes to queue
type ConcurrentPool struct {
	pool       []chan HttpRequest
	current    int // round-robin pool
	mu         sync.Mutex
	logger     *zap.SugaredLogger
	client     http.Client
	setHeaders SetHeaders
}

// NewConcurrentPool Creates instance of ConcurrentPool
func NewConcurrentPool(size int, timeout time.Duration, logger *zap.SugaredLogger) *ConcurrentPool {
	p := &ConcurrentPool{
		pool: make([]chan HttpRequest, size),
		client: http.Client{
			Timeout: timeout,
		},
		logger: logger,
		setHeaders: func(req *http.Request) {
			req.Header.Set("Content-Type", "application/json")
		},
	}

	for i := 0; i < size; i++ {
		// creates queue of [size]
		p.pool[i] = make(chan HttpRequest, 100000)
		pool := p.pool[i]
		go func() {
			for {
				select {
				case r := <-pool: // awaits requests from queue[i]
					switch r.Method {
					case http.MethodGet:
						res := p.get(r.Url, r.Payload, r.PoolNum)
						go r.CallBack(res)
					case http.MethodPost:
						res := p.post(r.Url, *r.Payload, r.PoolNum)
						go r.CallBack(res)
					}
				}
			}
		}()
	}

	return p
}

// DefineHeaders defines custom headers to all requests
func (s *ConcurrentPool) DefineHeaders(setHeaders SetHeaders) {
	baseSetHeaders := s.setHeaders
	s.setHeaders = func(req *http.Request) {
		baseSetHeaders(req)
		setHeaders(req)
	}
}

// Request sends http request get or post
func (s *ConcurrentPool) Request(req HttpRequest) {
	s.mu.Lock() // we need to lock for safely change current pool
	req.PoolNum = s.current
	s.pool[s.current] <- req
	s.current++
	if s.current >= len(s.pool) {
		s.current = 0
	}
	s.mu.Unlock()
}

// IsEmptyPool returns true if no requests in all pools
func (s *ConcurrentPool) IsEmptyPool() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < len(s.pool); i++ {
		if len(s.pool[i]) > 0 {
			return false
		}
	}
	return true
}

func (s *ConcurrentPool) get(url string, body *string, poolNum int) *http.Response {
	var req *http.Request
	var err error
	ctx, sgnl := context.WithTimeout(context.Background(), time.Second*120)
	defer sgnl()
	start := time.Now().UnixNano() / int64(time.Millisecond)
	if body != nil {
		s.logger.Debug("Request to Process API\t", poolNum, "\tMethod: GET", "\turl: ", url, "\tpayload: ", *body)
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewBuffer([]byte(*body)))
	} else {
		s.logger.Debug("Request to Process API\t Method: GET", "\turl: ", url, "\tpayload: nil")
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	}
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
	}
	s.setHeaders(req)
	r, err := s.client.Do(req)
	end := time.Now().UnixNano() / int64(time.Millisecond)
	s.logger.Debug("Request to Process API\t Method: GET", "\turl: ", url, "\tDONE in ", end-start, "ms")
	if err != nil || r != nil && r.StatusCode != http.StatusOK {
		if body != nil {
			s.logger.Error(err, "\tmethod:\tGET\tstatus: ", r.Status, "\turl: ", url, "\tbody: ", *body)
		} else {
			s.logger.Error(err, "\tmethod:\tGET\tstatus: ", r.Status, "\turl: ", url)
		}
	}
	return r
}

func (s *ConcurrentPool) post(url string, payload string, pool int) *http.Response {
	var jsonStr = []byte(payload)
	ctx, cncl := context.WithTimeout(context.Background(), time.Second*120)
	defer cncl()
	s.logger.Debug("Request to Process API\t", pool, "\tMethod: POST", "\turl: ", url, "\tpayload: ", payload)
	start := time.Now().UnixNano() / int64(time.Millisecond)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonStr))
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
	}
	s.setHeaders(req)
	r, err := s.client.Do(req)

	end := time.Now().UnixNano() / int64(time.Millisecond)
	s.logger.Debug("Request to Process API\t Method: POST", "\turl: ", url, "\tDONE in ", end-start, "ms")

	if err != nil || r.StatusCode != http.StatusOK {
		s.logger.Error(err, "\tmethod:\tPOST\tstatus: ", r.Status, "\turl: ", url, "\tpayload: ", payload)
		return nil
	}
	return r
}
