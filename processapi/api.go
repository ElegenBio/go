package processapi

import (
	"encoding/json"
	"fmt"
	"github.com/ElegenBio/go.git/common"
	http2 "github.com/ElegenBio/go.git/http"
	"go.uber.org/zap"
	"io"
	"net/http"
	"time"
)

type ApiResponse struct {
	Status int
	Data   interface{}
}

type ApiResponseCallback func(*ApiResponse)

type ProcessAPI struct {
	url    string
	apiKey string
	logger *zap.SugaredLogger
	pool   *http2.ConcurrentPool
}

func NewProcessAPI(url, apiKey string, timeout time.Duration, logger *zap.SugaredLogger) *ProcessAPI {
	api := &ProcessAPI{
		url:    url,
		apiKey: apiKey,
		logger: logger,
		pool:   http2.NewConcurrentPool(10, timeout, logger),
	}
	api.pool.DefineHeaders(api.setBasicHeaders)
	return api
}

func (s *ProcessAPI) GetUrl(path string) string {
	return fmt.Sprintf("%s%s", s.url, path)
}

func (s *ProcessAPI) FindUserAsync(callback ApiResponseCallback, email string) {
	payload := fmt.Sprintf("{\"email\": \"%s\"}", email)
	s.pool.Request(http2.HttpRequest{
		Method:  http.MethodPost,
		Url:     s.GetUrl("user/find"),
		Payload: &payload,
		CallBack: func(r *http.Response) {
			callback(s.parseResponse(r))
		},
	})
}

func (s *ProcessAPI) GetOrdersByDomainAsync(callback ApiResponseCallback, domainId string) {
	payload := fmt.Sprintf("{\"customerId\": \"%s\"}", domainId)
	s.pool.Request(http2.HttpRequest{
		Method:  http.MethodPost,
		Url:     s.GetUrl("/account/orders"),
		Payload: &payload,
		CallBack: func(r *http.Response) {
			callback(s.parseResponse(r))
		},
	})
}

func (s *ProcessAPI) GetOrdersByEmailAsync(callback ApiResponseCallback, email string) {
	payload := fmt.Sprintf("{\"userEmail\": \"%s\"}", email)
	s.pool.Request(http2.HttpRequest{
		Method:  http.MethodPost,
		Url:     s.GetUrl("/user/orders"),
		Payload: &payload,
		CallBack: func(r *http.Response) {
			callback(s.parseResponse(r))
		},
	})
}

func (s *ProcessAPI) GetOrderDetailsByIdAsync(callback ApiResponseCallback, orderId string) {
	payload := fmt.Sprintf("{\"orderId\": \"%s\"}", orderId)
	s.pool.Request(http2.HttpRequest{
		Method:  http.MethodPost,
		Url:     s.GetUrl("/orders/idRefined"),
		Payload: &payload,
		CallBack: func(r *http.Response) {
			callback(s.parseResponse(r))
		},
	})
}

func (s *ProcessAPI) HasActiveRequest() bool {
	return !s.pool.IsEmptyPool()
}

func (s *ProcessAPI) Await(callback common.Callback) {
	time.Sleep(time.Second * 10)
	for s.HasActiveRequest() {
		time.Sleep(time.Second * 1)
	}
	go callback()
}

func (s *ProcessAPI) parseResponse(r *http.Response) *ApiResponse {
	//	list response
	body, _ := io.ReadAll(r.Body)
	var arrData []interface{}
	err := json.Unmarshal(body, &arrData)
	if err == nil {
		status := r.StatusCode
		if len(arrData) > 1 && arrData[len(arrData)-1] != 200 {
			status = int(arrData[len(arrData)-1].(float64))
		}
		if len(arrData) == 1 || status != http.StatusOK {
			return &ApiResponse{
				Status: status,
				Data:   arrData[0],
			}
		} else if len(arrData) > 1 {
			d := arrData[0].(map[string]interface{})["data"]
			return &ApiResponse{
				Status: status,
				Data:   d,
			}
		}
	} else {
		var data interface{}
		err := json.Unmarshal(body, &data)
		if err == nil {
			return &ApiResponse{Status: r.StatusCode, Data: data}
		}
	}
	s.logger.Error("Response Unmarshal failed")
	return nil
}

func (s *ProcessAPI) setBasicHeaders(req *http.Request) {
	req.Header.Set("x-api-key", s.apiKey)
}