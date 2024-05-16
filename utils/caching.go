package utils

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"time"
)

type Caching struct {
	r      *redis.Client
	logger *zap.SugaredLogger
}

func NewCachingService(r *redis.Client, logger *zap.SugaredLogger) *Caching {
	return &Caching{
		r:      r,
		logger: logger,
	}
}

func (s *Caching) ClearAll() {
	s.logger.Info("Drop all cache!")
	s.r.FlushAll(context.Background())
}

type CachedArray struct {
	Data      []map[string]interface{} `json:"data"`
	UpdatedAt string                   `json:"updated_at"`
}

type CachedMap struct {
	Data      map[string]interface{} `json:"data"`
	UpdatedAt string                 `json:"updated_at"`
}

func (s *Caching) FromCacheArray(key string) CachedArray {
	cmd := s.r.Get(context.Background(), key)
	data := []byte(cmd.Val())
	var object CachedArray
	err := json.Unmarshal(data, &object)
	if err != nil {
		s.logger.Error(err)
	}
	return object
}

func (s *Caching) FromCacheMap(key string) CachedMap {
	cmd := s.r.Get(context.Background(), key)
	data := []byte(cmd.Val())
	var object CachedMap
	err := json.Unmarshal(data, &object)
	if err != nil {
		s.logger.Error(err)
	}
	return object
}

func (s *Caching) ToCache(key string, value interface{}) {
	data := make(map[string]interface{})
	data["data"] = value
	data["updated_at"] = time.Now().Format(time.RFC3339)
	jsonString, _ := json.Marshal(data)
	status := s.r.Set(context.Background(), key, jsonString, 0)
	_, err := status.Result()
	if err != nil {
		s.logger.Error(err)
	} else {
		s.logger.Info("Key Updated ", "Key ", key, "value ", value)
	}
}

func (s *Caching) Drop(key string) {
	s.logger.Info("Key Deleted ", "Key", key)
	status := s.r.Del(context.Background(), key)
	_, err := status.Result()
	if err != nil {
		s.logger.Error(err)
	}
}
