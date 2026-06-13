package server

import "os"

// Config stores runtime options for the HTTP server.
type Config struct {
	Port        string
	DatabaseDSN string
	WebDistDir  string
}

func LoadConfigFromEnv() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable"
	}

	webDistDir := os.Getenv("WEB_DIST_DIR")
	if webDistDir == "" {
		webDistDir = "web/dist"
	}

	return Config{
		Port:        port,
		DatabaseDSN: dsn,
		WebDistDir:  webDistDir,
	}
}
