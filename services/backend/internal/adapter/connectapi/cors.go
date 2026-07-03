package connectapi

import (
	"net/http"

	"github.com/rs/cors"
)

func NewCORS(allowedOrigins []string) func(http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{
			"Authorization", "Content-Type",
			"Connect-Protocol-Version", "Connect-Timeout-Ms",
		},
		ExposedHeaders:   []string{"Content-Type", "Connect-Protocol-Version"},
		AllowCredentials: true,
	})
	return c.Handler
}
