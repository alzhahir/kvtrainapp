package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://transit:transit@localhost:5432/kv_transit"
	}

	pool, err := Connect(databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := RunMigrations(pool); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/routes", handleRoutes(pool))
	mux.HandleFunc("GET /api/vehicles", handleVehicles(pool))
	mux.HandleFunc("GET /api/stops", handleStops(pool))
	mux.HandleFunc("GET /api/stations", handleStations(pool))
	mux.HandleFunc("GET /api/stations/{stop_id}/eta", handleStationETA(pool))
	mux.HandleFunc("GET /api/shapes", handleShapes(pool))
	mux.HandleFunc("GET /api/route-plan", handleRoutePlan(pool))
	mux.HandleFunc("GET /api/service-status", handleServiceStatus(pool))
	mux.HandleFunc("POST /api/admin/import", handleImport(pool))

	StartScheduler(pool)

	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(mux)))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
