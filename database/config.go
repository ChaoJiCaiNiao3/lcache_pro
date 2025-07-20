package database

type Config struct {
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
}

var DefaultConfig = &Config{
	RedisHost:     "localhost",
	RedisPort:     "6379",
	RedisPassword: "",
	RedisDB:       0,
}
