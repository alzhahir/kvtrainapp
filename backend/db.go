package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}
	return pool, nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS agencies (
    agency_id TEXT PRIMARY KEY,
    agency_name TEXT NOT NULL,
    agency_url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS routes (
    route_id TEXT PRIMARY KEY,
    agency_id TEXT REFERENCES agencies(agency_id),
    route_short_name TEXT,
    route_long_name TEXT,
    route_color TEXT,
    route_text_color TEXT,
    route_type INTEGER
);

CREATE TABLE IF NOT EXISTS trips (
    trip_id TEXT PRIMARY KEY,
    route_id TEXT REFERENCES routes(route_id),
    shape_id TEXT,
    direction_id INTEGER,
    service_id TEXT,
    trip_headsign TEXT
);

CREATE TABLE IF NOT EXISTS stops (
    stop_id TEXT PRIMARY KEY,
    stop_name TEXT NOT NULL,
    stop_lat DOUBLE PRECISION,
    stop_lon DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS frequencies (
    id SERIAL PRIMARY KEY,
    trip_id TEXT,
    start_time TEXT,
    end_time TEXT,
    headway_secs INTEGER
);
CREATE INDEX IF NOT EXISTS idx_frequencies_trip ON frequencies(trip_id);

CREATE TABLE IF NOT EXISTS calendar (
    service_id TEXT PRIMARY KEY,
    monday INTEGER DEFAULT 0,
    tuesday INTEGER DEFAULT 0,
    wednesday INTEGER DEFAULT 0,
    thursday INTEGER DEFAULT 0,
    friday INTEGER DEFAULT 0,
    saturday INTEGER DEFAULT 0,
    sunday INTEGER DEFAULT 0,
    start_date TEXT,
    end_date TEXT
);

CREATE TABLE IF NOT EXISTS stop_times (
    id SERIAL PRIMARY KEY,
    trip_id TEXT,
    stop_id TEXT,
    arrival_time TEXT,
    departure_time TEXT,
    stop_sequence INTEGER
);
CREATE INDEX IF NOT EXISTS idx_stop_times_trip ON stop_times(trip_id);

CREATE TABLE IF NOT EXISTS shapes (
    id SERIAL PRIMARY KEY,
    shape_id TEXT,
    shape_pt_lat DOUBLE PRECISION,
    shape_pt_lon DOUBLE PRECISION,
    shape_pt_sequence INTEGER
);

CREATE TABLE IF NOT EXISTS vehicle_positions (
    id SERIAL PRIMARY KEY,
    vehicle_id TEXT,
    trip_id TEXT,
    route_id TEXT,
    lat DOUBLE PRECISION,
    lon DOUBLE PRECISION,
    bearing REAL,
    speed REAL,
    delay_seconds INTEGER DEFAULT 0,
    fetched_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vp_fetched ON vehicle_positions(fetched_at DESC);
CREATE INDEX IF NOT EXISTS idx_vp_vehicle ON vehicle_positions(vehicle_id);
CREATE INDEX IF NOT EXISTS idx_stop_times_trip ON stop_times(trip_id);
CREATE INDEX IF NOT EXISTS idx_shapes_sid ON shapes(shape_id);
CREATE INDEX IF NOT EXISTS idx_trips_route ON trips(route_id);
CREATE INDEX IF NOT EXISTS idx_trips_shape ON trips(shape_id);

CREATE TABLE IF NOT EXISTS service_status (
    line_id TEXT PRIMARY KEY,
    line_name TEXT,
    status TEXT,
    remarks TEXT,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS import_log (
    id SERIAL PRIMARY KEY,
    agency TEXT,
    imported_at TIMESTAMP DEFAULT NOW(),
    status TEXT
);

-- ponytail: drop FK constraints from previous runs — GTFS data has orphans we don't control
ALTER TABLE IF EXISTS stop_times DROP CONSTRAINT IF EXISTS stop_times_trip_id_fkey;
ALTER TABLE IF EXISTS stop_times DROP CONSTRAINT IF EXISTS stop_times_stop_id_fkey;
ALTER TABLE IF EXISTS trips DROP CONSTRAINT IF EXISTS trips_route_id_fkey;
ALTER TABLE IF EXISTS routes DROP CONSTRAINT IF EXISTS routes_agency_id_fkey;
`

func RunMigrations(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), schemaSQL)
	return err
}
