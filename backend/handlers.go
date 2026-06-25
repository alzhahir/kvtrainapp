package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func handleRoutes(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(context.Background(),
			`SELECT route_id, agency_id, route_short_name, route_long_name,
			        route_color, route_text_color, route_type
			 FROM routes ORDER BY route_short_name`)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		var routes []Route
		for rows.Next() {
			var rt Route
			if err := rows.Scan(&rt.RouteID, &rt.AgencyID, &rt.RouteShortName,
				&rt.RouteLongName, &rt.RouteColor, &rt.RouteTextColor, &rt.RouteType); err != nil {
				continue
			}
			routes = append(routes, rt)
		}
		jsonResponse(w, routes)
	}
}

func handleVehicles(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(context.Background(),
			`SELECT DISTINCT ON (vehicle_id) vehicle_id, COALESCE(trip_id,''), COALESCE(route_id,''),
			        lat, lon, COALESCE(bearing,0), COALESCE(speed,0),
			        COALESCE(delay_seconds,0), fetched_at
			 FROM vehicle_positions
			 WHERE fetched_at > NOW() - INTERVAL '5 minutes'
			 ORDER BY vehicle_id, fetched_at DESC`)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		var vehicles []VehiclePosition
		for rows.Next() {
			var v VehiclePosition
			var fetchedAt time.Time
			if err := rows.Scan(&v.VehicleID, &v.TripID, &v.RouteID,
				&v.Lat, &v.Lon, &v.Bearing, &v.Speed, &v.DelaySeconds, &fetchedAt); err != nil {
				continue
			}
			v.FetchedAt = fetchedAt.Format(time.RFC3339)
			vehicles = append(vehicles, v)
		}

		jsonResponse(w, VehicleResponse{
			Vehicles: vehicles,
			Count:    len(vehicles),
			Time:     time.Now().Format(time.RFC3339),
		})
	}
}

func handleStops(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(context.Background(),
			`SELECT stop_id, stop_name, stop_lat, stop_lon FROM stops ORDER BY stop_name`)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		var stops []Stop
		for rows.Next() {
			var s Stop
			if err := rows.Scan(&s.StopID, &s.StopName, &s.StopLat, &s.StopLon); err != nil {
				continue
			}
			stops = append(stops, s)
		}
		jsonResponse(w, stops)
	}
}

func handleShapes(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ponytail: query shapes grouped by shape_id with route info
		rows, err := pool.Query(context.Background(),
			`SELECT s.shape_id, s.shape_pt_lat, s.shape_pt_lon, s.shape_pt_sequence,
			        COALESCE(t.route_id, '')
			 FROM shapes s
			 LEFT JOIN trips t ON s.shape_id = t.shape_id
			 ORDER BY s.shape_id, s.shape_pt_sequence`)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		shapeMap := make(map[string]*ShapeResponse)
		for rows.Next() {
			var shapeID string
			var lat, lon float64
			var seq int
			var routeID string
			if err := rows.Scan(&shapeID, &lat, &lon, &seq, &routeID); err != nil {
				continue
			}
			if _, ok := shapeMap[shapeID]; !ok {
				shapeMap[shapeID] = &ShapeResponse{
					ShapeID: shapeID,
					RouteID: routeID,
					Points:  []ShapePoint{},
				}
			}
			shapeMap[shapeID].Points = append(shapeMap[shapeID].Points, ShapePoint{Lat: lat, Lon: lon})
		}

		shapes := make([]ShapeResponse, 0, len(shapeMap))
		for _, s := range shapeMap {
			shapes = append(shapes, *s)
		}
		jsonResponse(w, shapes)
	}
}

func handleStations(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ponytail: array_agg with filter — aggregate multiple routes per stop
		rows, err := pool.Query(context.Background(),
			`SELECT s.stop_id, s.stop_name, s.stop_lat, s.stop_lon,
			        COALESCE(array_agg(DISTINCT r.route_id) FILTER (WHERE r.route_id IS NOT NULL), '{}') AS route_ids,
			        COALESCE(array_agg(DISTINCT r.route_long_name) FILTER (WHERE r.route_long_name IS NOT NULL), '{}') AS route_names,
			        COALESCE((array_agg(DISTINCT r.route_color) FILTER (WHERE r.route_color IS NOT NULL))[1], '') AS route_color
			 FROM stops s
			 JOIN stop_times st ON s.stop_id = st.stop_id
			 JOIN trips t ON st.trip_id = t.trip_id
			 JOIN routes r ON t.route_id = r.route_id
			 GROUP BY s.stop_id, s.stop_name, s.stop_lat, s.stop_lon
			 ORDER BY s.stop_name`)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		var stations []Station
		for rows.Next() {
			var st Station
			if err := rows.Scan(&st.StopID, &st.StopName, &st.StopLat, &st.StopLon,
				&st.RouteIDs, &st.RouteNames, &st.RouteColor); err != nil {
				continue
			}
			stations = append(stations, st)
		}
		jsonResponse(w, stations)
	}
}

func handleStationETA(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stopID := r.PathValue("stop_id")
		if stopID == "" {
			jsonError(w, "stop_id required", 400)
			return
		}

		dow := time.Now().Weekday()
		var dayCol string
		switch dow {
		case time.Sunday:
			dayCol = "c.sunday"
		case time.Monday:
			dayCol = "c.monday"
		case time.Tuesday:
			dayCol = "c.tuesday"
		case time.Wednesday:
			dayCol = "c.wednesday"
		case time.Thursday:
			dayCol = "c.thursday"
		case time.Friday:
			dayCol = "c.friday"
		case time.Saturday:
			dayCol = "c.saturday"
		}

		// ponytail: uses generate_series to expand frequency schedules; falls back to raw stop_times for trips without frequencies
		q := fmt.Sprintf(`
			WITH myt AS (
			  SELECT (EXTRACT(HOUR FROM (CURRENT_TIME + INTERVAL '8 hours')) * 3600 +
			          EXTRACT(MINUTE FROM (CURRENT_TIME + INTERVAL '8 hours')) * 60 +
			          EXTRACT(SECOND FROM (CURRENT_TIME + INTERVAL '8 hours')))::INTEGER AS sec
			), base AS (
			  SELECT DISTINCT
			    CAST(split_part(st.arrival_time,':',1) AS INTEGER) * 3600 +
			    CAST(split_part(st.arrival_time,':',2) AS INTEGER) * 60 +
			    CAST(split_part(st.arrival_time,':',3) AS INTEGER) AS arr_sec,
			    COALESCE(CAST(split_part(f.end_time,':',1) AS INTEGER) * 3600 +
			             CAST(split_part(f.end_time,':',2) AS INTEGER) * 60 +
			             CAST(split_part(f.end_time,':',3) AS INTEGER), 86400) AS end_sec,
			    f.headway_secs, t.trip_id, t.direction_id,
			    r.route_id, r.route_long_name, r.route_color,
			    COALESCE(t.trip_headsign, '') AS headsign
			  FROM stop_times st
			  JOIN trips t ON st.trip_id = t.trip_id
			  JOIN routes r ON t.route_id = r.route_id
			  LEFT JOIN calendar c ON t.service_id = c.service_id
			  LEFT JOIN frequencies f ON st.trip_id = f.trip_id
			  WHERE st.stop_id = $1
			    AND (%s = 1 OR c.service_id IS NULL)
			)
			SELECT
			  LPAD(((b.arr_sec + n.n * COALESCE(b.headway_secs, 86400)) / 3600)::text, 2, '0') || ':' ||
			  LPAD((((b.arr_sec + n.n * COALESCE(b.headway_secs, 86400)) %% 3600) / 60)::text, 2, '0') || ':' ||
			  LPAD(((b.arr_sec + n.n * COALESCE(b.headway_secs, 86400)) %% 60)::text, 2, '0') AS arrival_time,
			  b.route_id, b.route_long_name, b.route_color, b.trip_id, b.direction_id, b.headsign
			FROM base b, myt
			CROSS JOIN LATERAL generate_series(0, CASE WHEN b.headway_secs IS NULL THEN 0 ELSE 500 END) n
			WHERE b.arr_sec + n.n * COALESCE(b.headway_secs, 86400) > myt.sec
			  AND b.arr_sec + n.n * COALESCE(b.headway_secs, 86400) < b.end_sec + 3600
			ORDER BY b.arr_sec + n.n * COALESCE(b.headway_secs, 86400)
			LIMIT 10`, dayCol)

		rows, err := pool.Query(context.Background(), q, stopID)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		var etas []ETA
		for rows.Next() {
			var e ETA
			if err := rows.Scan(&e.ArrivalTime, &e.RouteID, &e.RouteName, &e.RouteColor, &e.TripID, &e.DirectionID, &e.Headsign); err != nil {
				continue
			}
			etas = append(etas, e)
		}
		if etas == nil {
			etas = []ETA{}
		}
		jsonResponse(w, etas)
	}
}

func handleRoutePlan(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		if from == "" || to == "" {
			jsonError(w, "from and to params required", 400)
			return
		}

		ctx := context.Background()
		var fromStop, toStop Stop
		pool.QueryRow(ctx, `SELECT stop_id, stop_name, stop_lat, stop_lon FROM stops WHERE stop_id = $1`, from).Scan(&fromStop.StopID, &fromStop.StopName, &fromStop.StopLat, &fromStop.StopLon)
		pool.QueryRow(ctx, `SELECT stop_id, stop_name, stop_lat, stop_lon FROM stops WHERE stop_id = $1`, to).Scan(&toStop.StopID, &toStop.StopName, &toStop.StopLat, &toStop.StopLon)

		// ponytail: direct routes first
		q := `
		SELECT DISTINCT ON (r.route_id)
		  r.route_id, r.route_long_name, r.route_color, t.direction_id,
		  COALESCE(t.shape_id, '') AS shape_id,
		  (st2.stop_sequence - st1.stop_sequence - 1) AS stops_between,
		  (CAST(split_part(st2.arrival_time,':',1) AS INTEGER) * 3600 +
		   CAST(split_part(st2.arrival_time,':',2) AS INTEGER) * 60 +
		   CAST(split_part(st2.arrival_time,':',3) AS INTEGER)) -
		  (CAST(split_part(st1.arrival_time,':',1) AS INTEGER) * 3600 +
		   CAST(split_part(st1.arrival_time,':',2) AS INTEGER) * 60 +
		   CAST(split_part(st1.arrival_time,':',3) AS INTEGER)) AS duration_sec
		FROM stop_times st1
		JOIN trips t ON st1.trip_id = t.trip_id
		JOIN routes r ON t.route_id = r.route_id
		JOIN stop_times st2 ON st1.trip_id = st2.trip_id
		WHERE st1.stop_id = $1 AND st2.stop_id = $2
		  AND st1.stop_sequence < st2.stop_sequence
		ORDER BY r.route_id, duration_sec
		LIMIT 5`

		var results []RoutePlanRoute
		rows, err := pool.Query(ctx, q, from, to)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		for rows.Next() {
			var r RoutePlanRoute
			if err := rows.Scan(&r.RouteID, &r.RouteName, &r.RouteColor, &r.DirectionID,
				&r.ShapeID, &r.StopsBetween, &r.DurationSec); err != nil {
				continue
			}
			leg := RouteLeg{
				RouteID: r.RouteID, RouteName: r.RouteName, RouteColor: r.RouteColor,
				DirectionID: r.DirectionID, StopsBetween: r.StopsBetween, DurationSec: r.DurationSec,
				ShapeID: r.ShapeID, FromStop: fromStop, ToStop: toStop,
				Stops: getRouteStops(ctx, pool, r.RouteID, fromStop.StopName, toStop.StopName),
			}
			r.Legs = []RouteLeg{leg}
			results = append(results, r)
		}
		rows.Close()

		// ponytail: 1-transfer routes when no direct route exists
		if len(results) == 0 {
			results = findTransferRoutes(ctx, pool, fromStop, toStop)
		}

		if results == nil {
			results = []RoutePlanRoute{}
		}
		jsonResponse(w, RoutePlanResult{Routes: results, FromStop: fromStop, ToStop: toStop})
	}
}

// ponytail: single-transfer routes between any two stations.
func findTransferRoutes(ctx context.Context, pool *pgxpool.Pool, fromStop, toStop Stop) []RoutePlanRoute {
	// Step 1: name-based transfers (same station name on different lines)
	xfers := findNamedTransfers(ctx, pool, fromStop, toStop)
	// Step 2: proximity-based fallback (connected stations with different names, e.g. MERDEKA ↔ PLAZA RAKYAT)
	xfers = append(xfers, findNearbyTransfers(ctx, pool, fromStop, toStop)...)

	if len(xfers) == 0 {
		return nil
	}

	// Deduplicate by (leg1_route_id, leg2_route_id, transfer_stop_name)
	seen := make(map[string]bool)
	var deduped []xferInfo
	for _, x := range xfers {
		key := x.leg1RouteID + "|" + x.leg2RouteID + "|" + x.stopName
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, x)
	}

	var results []RoutePlanRoute
	for _, x := range deduped {
		xferStop := Stop{StopID: x.stopID, StopName: x.stopName, StopLat: x.stopLat, StopLon: x.stopLon}
		xferStop2 := Stop{StopID: x.stop2ID, StopName: x.stop2Name, StopLat: x.stop2Lat, StopLon: x.stop2Lon}

		var legs []RouteLeg
		// ponytail: skip zero-stop leg when transfer is at origin station
		if fromStop.StopName != x.stopName {
			leg1 := RouteLeg{
				RouteID: x.leg1RouteID, RouteName: x.leg1Name, RouteColor: x.leg1Color,
				DirectionID: 0, FromStop: fromStop, ToStop: xferStop,
				Stops: getRouteStops(ctx, pool, x.leg1RouteID, fromStop.StopName, x.stopName),
			}
			if leg1.Stops != nil {
				leg1.StopsBetween = len(leg1.Stops) - 1
			}
			legs = append(legs, leg1)
		}

		leg2 := RouteLeg{
			RouteID: x.leg2RouteID, RouteName: x.leg2Name, RouteColor: x.leg2Color,
			DirectionID: 0, FromStop: xferStop2, ToStop: toStop,
			Stops: getRouteStops(ctx, pool, x.leg2RouteID, x.stop2Name, toStop.StopName),
		}
		if leg2.Stops != nil {
			leg2.StopsBetween = len(leg2.Stops) - 1
		}
		legs = append(legs, leg2)

		results = append(results, RoutePlanRoute{
			Legs:       legs,
			TransferAt: &xferStop,
		})
	}
	// ponytail: shortest route first by total stop count
	sort.Slice(results, func(i, j int) bool {
		totalI, totalJ := 0, 0
		for _, l := range results[i].Legs {
			totalI += l.StopsBetween
		}
		for _, l := range results[j].Legs {
			totalJ += l.StopsBetween
		}
		return totalI < totalJ
	})
	return results
}

type xferInfo struct {
	stopID, stopName string
	stopLat, stopLon float64
	stop2ID, stop2Name string
	stop2Lat, stop2Lon float64
	leg1RouteID, leg1Name, leg1Color string
	leg2RouteID, leg2Name, leg2Color string
}

func findNamedTransfers(ctx context.Context, pool *pgxpool.Pool, fromStop, toStop Stop) []xferInfo {
	q := `
	WITH from_route_ids AS (
	  SELECT DISTINCT t.route_id FROM stop_times st JOIN trips t ON st.trip_id = t.trip_id WHERE st.stop_id = $1
	),
	to_route_ids AS (
	  SELECT DISTINCT t.route_id FROM stop_times st JOIN trips t ON st.trip_id = t.trip_id WHERE st.stop_id = $2
	)
	SELECT DISTINCT ON (s.stop_name)
	  s.stop_id, s.stop_name, s.stop_lat, s.stop_lon,
	  fr.route_id, r1.route_long_name, r1.route_color,
	  tr.route_id, r2.route_long_name, r2.route_color
	FROM (
	  SELECT DISTINCT s.stop_name, t.route_id
	  FROM stop_times st JOIN trips t ON st.trip_id = t.trip_id JOIN stops s ON st.stop_id = s.stop_id
	  WHERE t.route_id IN (SELECT route_id FROM from_route_ids) AND st.stop_id NOT IN ($1, $2)
	) fr
	JOIN (
	  SELECT DISTINCT s.stop_name, t.route_id
	  FROM stop_times st JOIN trips t ON st.trip_id = t.trip_id JOIN stops s ON st.stop_id = s.stop_id
	  WHERE t.route_id IN (SELECT route_id FROM to_route_ids) AND st.stop_id NOT IN ($1, $2)
	) tr ON fr.stop_name = tr.stop_name AND fr.route_id != tr.route_id
	JOIN stops s ON s.stop_name = fr.stop_name
	JOIN routes r1 ON fr.route_id = r1.route_id
	JOIN routes r2 ON tr.route_id = r2.route_id
	ORDER BY s.stop_name, fr.route_id, tr.route_id`

	rows, err := pool.Query(ctx, q, fromStop.StopID, toStop.StopID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var xfers []xferInfo
	for rows.Next() {
		var x xferInfo
		if err := rows.Scan(&x.stopID, &x.stopName, &x.stopLat, &x.stopLon,
			&x.leg1RouteID, &x.leg1Name, &x.leg1Color,
			&x.leg2RouteID, &x.leg2Name, &x.leg2Color); err != nil {
			continue
		}
		x.stop2ID, x.stop2Name, x.stop2Lat, x.stop2Lon = x.stopID, x.stopName, x.stopLat, x.stopLon
		xfers = append(xfers, x)
	}
	return xfers
}

// ponytail: proximity-based transfer finder — stations ≤200m apart on different lines
func findNearbyTransfers(ctx context.Context, pool *pgxpool.Pool, fromStop, toStop Stop) []xferInfo {
	q := `
	WITH from_route_ids AS (
	  SELECT DISTINCT t.route_id FROM stop_times st JOIN trips t ON st.trip_id = t.trip_id WHERE st.stop_id = $1
	),
	to_route_ids AS (
	  SELECT DISTINCT t.route_id FROM stop_times st JOIN trips t ON st.trip_id = t.trip_id WHERE st.stop_id = $2
	)
	SELECT DISTINCT ON (fs.route_id, ts.route_id)
	  fs.stop_id, fs.stop_name, fs.stop_lat, fs.stop_lon,
	  ts.stop_id, ts.stop_name, ts.stop_lat, ts.stop_lon,
	  fs.route_id, r1.route_long_name, r1.route_color,
	  ts.route_id, r2.route_long_name, r2.route_color
	FROM (
	  SELECT DISTINCT s.stop_id, s.stop_name, s.stop_lat, s.stop_lon, t.route_id
	  FROM stop_times st JOIN trips t ON st.trip_id = t.trip_id JOIN stops s ON st.stop_id = s.stop_id
	  WHERE t.route_id IN (SELECT route_id FROM from_route_ids)
	) fs
	JOIN (
	  SELECT DISTINCT s.stop_id, s.stop_name, s.stop_lat, s.stop_lon, t.route_id
	  FROM stop_times st JOIN trips t ON st.trip_id = t.trip_id JOIN stops s ON st.stop_id = s.stop_id
	  WHERE t.route_id IN (SELECT route_id FROM to_route_ids)
	) ts ON fs.route_id != ts.route_id AND fs.stop_name != ts.stop_name
	  AND ABS(fs.stop_lat - ts.stop_lat) < 0.003
	  AND ABS(fs.stop_lon - ts.stop_lon) < 0.003
	JOIN routes r1 ON fs.route_id = r1.route_id
	JOIN routes r2 ON ts.route_id = r2.route_id
	ORDER BY fs.route_id, ts.route_id, fs.stop_name
	LIMIT 5`

	rows, err := pool.Query(ctx, q, fromStop.StopID, toStop.StopID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var xfers []xferInfo
	for rows.Next() {
		var x xferInfo
		if err := rows.Scan(&x.stopID, &x.stopName, &x.stopLat, &x.stopLon,
			&x.stop2ID, &x.stop2Name, &x.stop2Lat, &x.stop2Lon,
			&x.leg1RouteID, &x.leg1Name, &x.leg1Color,
			&x.leg2RouteID, &x.leg2Name, &x.leg2Color); err != nil {
			continue
		}
		xfers = append(xfers, x)
	}
	return xfers
}

// ponytail: returns stop names in order between two stations on a route
func getRouteStops(ctx context.Context, pool *pgxpool.Pool, routeID, fromName, toName string) []string {
	var tripID string
	err := pool.QueryRow(ctx, `
		SELECT t.trip_id FROM stop_times st1
		JOIN trips t ON st1.trip_id = t.trip_id AND t.route_id = $1
		JOIN stop_times st2 ON st1.trip_id = st2.trip_id
		JOIN stops s1 ON st1.stop_id = s1.stop_id
		JOIN stops s2 ON st2.stop_id = s2.stop_id
		WHERE s1.stop_name = $2 AND s2.stop_name = $3
		AND st1.stop_sequence < st2.stop_sequence
		LIMIT 1`, routeID, fromName, toName).Scan(&tripID)
	if err != nil {
		return nil
	}

	rows, err := pool.Query(ctx, `
		SELECT s.stop_name FROM stop_times st
		JOIN stops s ON st.stop_id = s.stop_id
		WHERE st.trip_id = $1
		GROUP BY s.stop_name
		ORDER BY MIN(st.stop_sequence)`, tripID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var allStops []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		allStops = append(allStops, name)
	}

	fromIdx, toIdx := -1, -1
	for i, name := range allStops {
		if name == fromName && fromIdx == -1 {
			fromIdx = i
		}
		if name == toName {
			toIdx = i
		}
	}
	if fromIdx == -1 || toIdx == -1 || fromIdx >= toIdx {
		return nil
	}
	return allStops[fromIdx : toIdx+1]
}

func handleServiceStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(context.Background(),
			`SELECT line_id, line_name, status, remarks, updated_at
			 FROM service_status ORDER BY line_name`)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		var statuses []ServiceStatus
		for rows.Next() {
			var s ServiceStatus
			var updatedAt time.Time
			if err := rows.Scan(&s.LineID, &s.LineName, &s.Status, &s.Remarks, &updatedAt); err != nil {
				continue
			}
			s.UpdatedAt = updatedAt.Format(time.RFC3339)
			statuses = append(statuses, s)
		}

		jsonResponse(w, ServiceStatusResponse{
			Statuses: statuses,
			Time:     time.Now().Format(time.RFC3339),
		})
	}
}

func handleImport(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		go func() {
			for _, a := range importAgencies {
				if err := ImportStaticURL(pool, a.name, a.url); err != nil {
					log.Printf("import %s: %v", a.name, err)
				}
			}
		}()
		jsonResponse(w, map[string]string{"status": "import started"})
	}
}
