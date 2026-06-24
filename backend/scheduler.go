package main

import (
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

func StartScheduler(pool *pgxpool.Pool) {
	c := cron.New()

	// Re-import GTFS static daily
	c.AddFunc("@daily", func() {
		log.Println("Daily GTFS import...")
		for _, a := range importAgencies {
			if err := ImportStaticURL(pool, a.name, a.url); err != nil {
				log.Printf("import %s: %v", a.name, err)
			}
		}
	})

	// Fetch realtime vehicle positions every 30s
	c.AddFunc("@every 30s", func() {
		if err := FetchRealtime(pool); err != nil {
			log.Printf("realtime fetch: %v", err)
		}
	})

	// // ponytail: disabled — myrapid.com.my is an SPA, can't scrape.
	// c.AddFunc("@every 5m", func() {
	// 	log.Println("fetching service status...")
	// 	if err := FetchServiceStatus(pool); err != nil {
	// 		log.Printf("service status fetch: %v", err)
	// 	}
	// })

	c.Start()

	// Run initial import + service status after DB is ready
	time.AfterFunc(3*time.Second, func() {
		log.Println("Initial GTFS import starting...")
		for _, a := range importAgencies {
			if err := ImportStaticURL(pool, a.name, a.url); err != nil {
				log.Printf("initial import %s: %v", a.name, err)
			}
		}
		log.Println("Initial import done")
		// ponytail: FetchServiceStatus disabled — myrapid.com.my uses JS SPA, can't scrape.
	// Re-enable when a proper API endpoint is available.
	// if err := FetchServiceStatus(pool); err != nil {
	// 	log.Printf("initial service status fetch: %v", err)
	// }
	})
}
