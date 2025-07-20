package database

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

type Redis struct {
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	RedisClient   *redis.Client
}

func NewRedis(config *Config) *Redis {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     DefaultConfig.RedisHost + ":" + DefaultConfig.RedisPort,
		Password: DefaultConfig.RedisPassword,
		DB:       DefaultConfig.RedisDB,
	})
	return &Redis{
		RedisHost:     DefaultConfig.RedisHost,
		RedisPort:     DefaultConfig.RedisPort,
		RedisPassword: DefaultConfig.RedisPassword,
		RedisDB:       DefaultConfig.RedisDB,
		RedisClient:   redisClient,
	}
}

func (r *Redis) Get(key string) (string, error) {
	return r.RedisClient.Get(context.Background(), key).Result()
}

func (r *Redis) Set(key string, value string) error {
	return r.RedisClient.Set(context.Background(), key, value, 0).Err()
}

func (r *Redis) Delete(key string) error {
	return r.RedisClient.Del(context.Background(), key).Err()
}
