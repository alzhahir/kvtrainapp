package main

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"

	gtfsrt "kv-transit/proto"

	"github.com/PuerkitoBio/goquery"
	"github.com/jackc/pgx/v5/pgxpool"
	gproto "google.golang.org/protobuf/proto"
)

const baseURL = "https://api.data.gov.my"

var agencies = []string{"prasarana", "ktmb"}

var realtimeURLs = []string{
	"https://api.data.gov.my/gtfs-realtime/vehicle-position/prasarana?category=rapid-bus-kl",
	"https://api.data.gov.my/gtfs-realtime/vehicle-position/prasarana?category=rapid-bus-mrtfeeder",
	"https://api.data.gov.my/gtfs-realtime/vehicle-position/ktmb",
}

// ---------------------------------------------------------------------------
// GTFS Static Import
// ---------------------------------------------------------------------------

var importAgencies = []struct {
	name string
	url  string
}{
	{"prasarana-rail", baseURL + "/gtfs-static/prasarana?category=rapid-rail-kl"},
	{"ktmb", baseURL + "/gtfs-static/ktmb"},
}

func ImportStaticURL(pool *pgxpool.Pool, name, url string) error {
	log.Printf("Importing GTFS static: %s", name)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", name, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	z, err := zip.NewReader(strings.NewReader(string(body)), int64(len(body)))
	if err != nil {
		return fmt.Errorf("zip %s: %w", name, err)
	}

	files := make(map[string]*zip.File)
	for _, f := range z.File {
		files[f.Name] = f
	}

	if f, ok := files["agency.txt"]; ok {
		importAgency(pool, f)
	}
	if f, ok := files["routes.txt"]; ok {
		importRoutes(pool, f)
	}
	if f, ok := files["trips.txt"]; ok {
		importTrips(pool, f)
	}
	if f, ok := files["stops.txt"]; ok {
		importStops(pool, f)
	}
	if f, ok := files["stop_times.txt"]; ok {
		importStopTimes(pool, f)
	}
	if f, ok := files["shapes.txt"]; ok {
		importShapes(pool, f)
	}
	if f, ok := files["calendar.txt"]; ok {
		importCalendar(pool, f)
	}
	if f, ok := files["frequencies.txt"]; ok {
		importFrequencies(pool, f)
	}

	log.Printf("Import complete: %s", name)
	return nil
}

func ImportStatic(pool *pgxpool.Pool, agency string) error {
	// ponytail: only supports ktmb; prasarana needs category param, use ImportStaticURL directly
	url := fmt.Sprintf("%s/gtfs-static/%s", baseURL, agency)
	return ImportStaticURL(pool, agency, url)
}

func readCSV(f *zip.File) ([]map[string]string, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, nil
	}

	headers := make([]string, len(records[0]))
	for i, h := range records[0] {
		headers[i] = strings.TrimLeft(h, "\ufeff\uFEFF") // strip BOM
	}
	var rows []map[string]string
	for _, row := range records[1:] {
		m := make(map[string]string)
		for i, h := range headers {
			if i < len(row) {
				m[h] = row[i]
			}
		}
		rows = append(rows, m)
	}
	return rows, nil
}

func importAgency(pool *pgxpool.Pool, f *zip.File) {
	rows, err := readCSV(f)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, r := range rows {
		pool.Exec(context.Background(),
			`INSERT INTO agencies (agency_id, agency_name, agency_url)
			 VALUES ($1, $2, $3) ON CONFLICT (agency_id) DO NOTHING`,
			r["agency_id"], r["agency_name"], r["agency_url"])
	}
}

func importRoutes(pool *pgxpool.Pool, f *zip.File) {
	rows, err := readCSV(f)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, r := range rows {
		rt := 0
		if v, ok := r["route_type"]; ok {
			rt, _ = strconv.Atoi(v)
		}
		pool.Exec(context.Background(),
			`INSERT INTO routes (route_id, agency_id, route_short_name, route_long_name, route_color, route_text_color, route_type)
			 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (route_id) DO UPDATE SET route_long_name=EXCLUDED.route_long_name`,
			r["route_id"], r["agency_id"], r["route_short_name"], r["route_long_name"],
			r["route_color"], r["route_text_color"], rt)
	}
}

func importTrips(pool *pgxpool.Pool, f *zip.File) {
	rows, err := readCSV(f)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, r := range rows {
		dir := 0
		if v, ok := r["direction_id"]; ok {
			dir, _ = strconv.Atoi(v)
		}
		pool.Exec(context.Background(),
			`INSERT INTO trips (trip_id, route_id, shape_id, direction_id, service_id)
			 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (trip_id) DO NOTHING`,
			r["trip_id"], r["route_id"], r["shape_id"], dir, r["service_id"])
	}
}

func importStops(pool *pgxpool.Pool, f *zip.File) {
	rows, err := readCSV(f)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, r := range rows {
		lat, _ := strconv.ParseFloat(r["stop_lat"], 64)
		lon, _ := strconv.ParseFloat(r["stop_lon"], 64)
		pool.Exec(context.Background(),
			`INSERT INTO stops (stop_id, stop_name, stop_lat, stop_lon)
			 VALUES ($1,$2,$3,$4) ON CONFLICT (stop_id) DO UPDATE SET stop_name=EXCLUDED.stop_name`,
			r["stop_id"], r["stop_name"], lat, lon)
	}
}

func importFrequencies(pool *pgxpool.Pool, f *zip.File) {
	rows, err := readCSV(f)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, r := range rows {
		hs, _ := strconv.Atoi(r["headway_secs"])
		pool.Exec(context.Background(),
			`INSERT INTO frequencies (trip_id, start_time, end_time, headway_secs)
			 VALUES ($1,$2,$3,$4)`,
			r["trip_id"], r["start_time"], r["end_time"], hs)
	}
}

func importCalendar(pool *pgxpool.Pool, f *zip.File) {
	rows, err := readCSV(f)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, r := range rows {
		m, _ := strconv.Atoi(r["monday"])
		t, _ := strconv.Atoi(r["tuesday"])
		w, _ := strconv.Atoi(r["wednesday"])
		th, _ := strconv.Atoi(r["thursday"])
		fr, _ := strconv.Atoi(r["friday"])
		sa, _ := strconv.Atoi(r["saturday"])
		su, _ := strconv.Atoi(r["sunday"])
		pool.Exec(context.Background(),
			`INSERT INTO calendar (service_id, monday, tuesday, wednesday, thursday, friday, saturday, sunday, start_date, end_date)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (service_id) DO UPDATE SET
			 monday=EXCLUDED.monday, tuesday=EXCLUDED.tuesday, wednesday=EXCLUDED.wednesday,
			 thursday=EXCLUDED.thursday, friday=EXCLUDED.friday, saturday=EXCLUDED.saturday,
			 sunday=EXCLUDED.sunday`,
			r["service_id"], m, t, w, th, fr, sa, su, r["start_date"], r["end_date"])
	}
}

// ponytail: sequential inserts for simplicity. Batch insert if >10k rows become slow.
func importStopTimes(pool *pgxpool.Pool, f *zip.File) {
	rows, err := readCSV(f)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, r := range rows {
		seq, _ := strconv.Atoi(r["stop_sequence"])
		pool.Exec(context.Background(),
			`INSERT INTO stop_times (trip_id, stop_id, arrival_time, departure_time, stop_sequence)
			 VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			r["trip_id"], r["stop_id"], r["arrival_time"], r["departure_time"], seq)
	}
}

func importShapes(pool *pgxpool.Pool, f *zip.File) {
	rows, err := readCSV(f)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, r := range rows {
		lat, _ := strconv.ParseFloat(r["shape_pt_lat"], 64)
		lon, _ := strconv.ParseFloat(r["shape_pt_lon"], 64)
		seq, _ := strconv.Atoi(r["shape_pt_sequence"])
		pool.Exec(context.Background(),
			`INSERT INTO shapes (shape_id, shape_pt_lat, shape_pt_lon, shape_pt_sequence)
			 VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			r["shape_id"], lat, lon, seq)
	}
}

// ---------------------------------------------------------------------------
// GTFS Realtime Fetch
// ---------------------------------------------------------------------------

func FetchRealtime(pool *pgxpool.Pool) error {
	for _, url := range realtimeURLs {
		if err := fetchAndStore(pool, url); err != nil {
			log.Printf("realtime fetch error %s: %v", url, err)
		}
	}
	return nil
}

func fetchAndStore(pool *pgxpool.Pool, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	feed := &gtfsrt.FeedMessage{}
	if err := gproto.Unmarshal(data, feed); err != nil {
		return fmt.Errorf("protobuf unmarshal: %w", err)
	}

	now := time.Now()
	for _, entity := range feed.Entity {
		vp := entity.GetVehicle()
		if vp == nil {
			continue
		}

		tripID := ""
		routeID := ""
		if t := vp.GetTrip(); t != nil {
			tripID = t.GetTripId()
			routeID = t.GetRouteId()
		}

		vehicleID := ""
		if v := vp.GetVehicle(); v != nil {
			vehicleID = v.GetId()
		}

		pos := vp.GetPosition()
		if pos == nil {
			continue
		}

		delay := calculateDelay(tripID, float64(pos.GetLatitude()), float64(pos.GetLongitude()), pool)

		pool.Exec(context.Background(),
			`INSERT INTO vehicle_positions
			     (vehicle_id, trip_id, route_id, lat, lon, bearing, speed, delay_seconds, fetched_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			vehicleID, tripID, routeID,
			float64(pos.GetLatitude()), float64(pos.GetLongitude()),
			float64(pos.GetBearing()), float64(pos.GetSpeed()),
			delay, now)
	}

	return nil
}

// ponytail: simple delay estimation – compares current position to nearest stop schedule
func calculateDelay(tripID string, lat, lon float64, pool *pgxpool.Pool) int {
	if tripID == "" {
		return 0
	}

	nowDur := nowDuration()
	var closestDiff time.Duration = 30 * time.Minute // if nothing is within 30 min, treat as unknown

	rows, err := pool.Query(context.Background(),
		`SELECT st.arrival_time, s.stop_lat, s.stop_lon
		 FROM stop_times st
		 JOIN stops s ON st.stop_id = s.stop_id
		 WHERE st.trip_id = $1
		 ORDER BY st.stop_sequence`, tripID)
	if err != nil {
		return 0
	}
	defer rows.Close()

	for rows.Next() {
		var arrival string
		var slat, slon float64
		if err := rows.Scan(&arrival, &slat, &slon); err != nil {
			continue
		}

		schedDur := parseGTFSTime(arrival)
		diff := nowDur - schedDur

		// match: we want the stop the vehicle should have ALREADY passed
		// by comparing scheduled time vs current time
		if diff > 0 && diff < closestDiff {
			closestDiff = diff
		}
	}

	if closestDiff < 10*time.Minute {
		return 0 // on time
	}
	return int(closestDiff.Seconds())
}

// ---------------------------------------------------------------------------
// Service Status Scraper
// ---------------------------------------------------------------------------

func FetchServiceStatus(pool *pgxpool.Pool) error {
	url := "https://myrapid.com.my/service-status/"
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch status page: %w", err)
	}
	defer resp.Body.Close()

	// ponytail: myrapid.com.my is an SPA (returns "Loading" or 403).
	// Scraping won't work. Will re-enable when they provide a public API.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status page returned HTTP %d (expected 200)", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("parse html: %w", err)
	}

	now := time.Now()
	doc.Find("tbody tr[data-row_id]").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 6 {
			return
		}
		name := strings.TrimSpace(cells.Eq(0).Text())
		status := strings.TrimSpace(cells.Eq(2).Text())
		remarks := strings.TrimSpace(cells.Eq(3).Text())
		lineID := strings.TrimSpace(cells.Eq(5).Text())

		if name == "" {
			return
		}

		pool.Exec(context.Background(),
			`INSERT INTO service_status (line_id, line_name, status, remarks, updated_at)
			 VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (line_id) DO UPDATE SET
			     line_name=EXCLUDED.line_name, status=EXCLUDED.status,
			     remarks=EXCLUDED.remarks, updated_at=EXCLUDED.updated_at`,
			lineID, name, status, remarks, now)
	})

	return nil
}
