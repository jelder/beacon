package main

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	. "github.com/jelder/env"
	neturl "net/url"
	"time"
)

var ENV EnvMap

func init() {
	ENV = MustLoadEnv()
}

func listenAddress() string {
	return ":" + ENV.Get("PORT", "8080")
}

func redisConfig() (string, string) {
	redis_provider := ENV["REDIS_PROVIDER"]
	if redis_provider == "" {
		redis_provider = "OPENREDIS_URL"
	}
	string := ENV[redis_provider]
	if string != "" {
		url, err := neturl.Parse(string)
		password := ""
		if err != nil {
			panic(err)
		}
		if url.User != nil {
			password, _ = url.User.Password()
		}
		return url.Host, password
	} else {
		return "127.0.0.1:6379", ""
	}
}

func redisSetup(server, password string) *redis.Pool {
	fmt.Println("Connecting to Redis on", server, password)
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
