// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"errors"
	"time"

	"gopkg.in/go-redis/cache.v1"
	"gopkg.in/redis.v3"
	"gopkg.in/vmihailenco/msgpack.v2"

	"github.com/Sirupsen/logrus"
	"github.com/TeamFairmont/boltengine/commandprocess"
)

// SetupCache uses the current config to connect and setup a cache system
func (engine *Engine) SetupCache() error {
	if engine.Config.Cache.Type == "" {
		return nil
	} else if engine.Config.Cache.Type == "redis" {
		timeout := time.Duration(engine.Config.Cache.TimeoutMs) * time.Millisecond

		ring := redis.NewRing(&redis.RingOptions{
			Addrs: map[string]string{
				"server1": engine.Config.Cache.Host,
			},
			Password: engine.Config.Cache.Pass,

			DialTimeout:  timeout,
			ReadTimeout:  timeout,
			WriteTimeout: timeout,
		})

		codec := &cache.Codec{
			Ring: ring,

			Marshal: func(v interface{}) ([]byte, error) {
				return msgpack.Marshal(v)
			},
			Unmarshal: func(b []byte, v interface{}) error {
				return msgpack.Unmarshal(b, v)
			},
		}

		engine.cacheCodec = codec
		return nil
	} else {
		return errors.New("Config error: Unsupported cache type")
	}
}

// SetCacheItem adds an item to cache, forces non-local cache use
func (engine *Engine) SetCacheItem(apicall string, inputjson string, value string, expiration time.Duration) error {
	key := assembleCacheKey(apicall, inputjson)
	if engine.cacheCodec != nil {
		item := &cache.Item{
			Key:               key,
			Object:            value,
			Expiration:        expiration,
			DisableLocalCache: true,
		}
		res := engine.cacheCodec.Set(item)
		return res
	}
	return nil
}

// GetCacheItem tries to get an item from cache if possible
func (engine *Engine) GetCacheItem(apicall string, inputjson string) (string, error) {
	key := assembleCacheKey(apicall, inputjson)
	if engine.cacheCodec != nil {
		var value string
		err := engine.cacheCodec.Get(key, &value)
		return value, err
	}
	return "", errors.New("No cache enabled")
}

// DelCacheItem attempts to force-delete a cached key
func (engine *Engine) DelCacheItem(apicall string, inputjson string) error {
	key := assembleCacheKey(apicall, inputjson)
	if engine.cacheCodec != nil {
		err := engine.cacheCodec.Delete(key)
		return err
	}
	return nil
}

// CacheCallResult sets the cache for an commandprocess if applicable
func (engine *Engine) CacheCallResult(cp *commandprocess.CommandProcess) error {
	if engine.cacheCodec != nil {
		if cp.APICall.Cache.Enabled {
			inputstr := cp.InitialInputString //issue #40, change to use a snapshot/copy of initial input in case workers accidentally change it
			retval := cp.Payload.Path("return_value").String()
			err := engine.SetCacheItem(engine.Config.Engine.Advanced.QueuePrefix+cp.InitialCommand, inputstr, retval, cp.APICall.Cache.ExpirationTime)
			if err == nil {
				engine.LogInfo("cache_set", logrus.Fields{"id": cp.ID, "command": cp.InitialCommand, "input": inputstr}, "Cache set")
			} else {
				engine.LogWarn("cache_error", nil, err.Error())
			}
			return err
		}
	}
	return nil
}

// assembleCacheKey combines an api call name with the input params for use as a cache key name
func assembleCacheKey(apicall, inputjson string) string {
	return apicall + "$$$" + inputjson
}
