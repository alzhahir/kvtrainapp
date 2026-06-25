package main

import (
	"fmt"
	"time"
)

type Agency struct {
	AgencyID   string `json:"agency_id"`
	AgencyName string `json:"agency_name"`
	AgencyURL  string `json:"agency_url"`
}

type Route struct {
	RouteID        string `json:"route_id"`
	AgencyID       string `json:"agency_id"`
	RouteShortName string `json:"route_short_name"`
	RouteLongName  string `json:"route_long_name"`
	RouteColor     string `json:"route_color"`
	RouteTextColor string `json:"route_text_color"`
	RouteType      int    `json:"route_type"`
}

type Trip struct {
	TripID      string `json:"trip_id"`
	RouteID     string `json:"route_id"`
	ShapeID     string `json:"shape_id"`
	DirectionID int    `json:"direction_id"`
	ServiceID   string `json:"service_id"`
}

type Stop struct {
	StopID   string  `json:"stop_id"`
	StopName string  `json:"stop_name"`
	StopLat  float64 `json:"stop_lat"`
	StopLon  float64 `json:"stop_lon"`
}

type StopTime struct {
	ID            int    `json:"id"`
	TripID        string `json:"trip_id"`
	StopID        string `json:"stop_id"`
	ArrivalTime   string `json:"arrival_time"`
	DepartureTime string `json:"departure_time"`
	StopSequence  int    `json:"stop_sequence"`
}

type ShapePoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type ShapeResponse struct {
	ShapeID string       `json:"shape_id"`
	RouteID string       `json:"route_id"`
	Points  []ShapePoint `json:"points"`
}

type VehiclePosition struct {
	ID           int     `json:"id"`
	VehicleID    string  `json:"vehicle_id"`
	TripID       string  `json:"trip_id,omitempty"`
	RouteID      string  `json:"route_id,omitempty"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Bearing      float32 `json:"bearing"`
	Speed        float32 `json:"speed"`
	DelaySeconds int     `json:"delay_seconds"`
	FetchedAt    string  `json:"fetched_at"`
}

type ServiceStatus struct {
	LineID    string `json:"line_id"`
	LineName  string `json:"line_name"`
	Status    string `json:"status"`
	Remarks   string `json:"remarks"`
	UpdatedAt string `json:"updated_at"`
}

type Station struct {
	StopID     string   `json:"stop_id"`
	StopName   string   `json:"stop_name"`
	StopLat    float64  `json:"stop_lat"`
	StopLon    float64  `json:"stop_lon"`
	RouteIDs   []string `json:"route_ids"`
	RouteNames []string `json:"route_names"`
	RouteColor string   `json:"route_color"`
}

type ETA struct {
	ArrivalTime string `json:"arrival_time"`
	RouteID     string `json:"route_id"`
	RouteName   string `json:"route_name"`
	RouteColor  string `json:"route_color"`
	TripID      string `json:"trip_id"`
	DirectionID int    `json:"direction_id"`
	Headsign    string `json:"headsign"`
}

type VehicleResponse struct {
	Vehicles []VehiclePosition `json:"vehicles"`
	Count    int               `json:"count"`
	Time     string            `json:"time"`
}

type ServiceStatusResponse struct {
	Statuses []ServiceStatus `json:"statuses"`
	Time     string          `json:"time"`
}

type RouteLeg struct {
	RouteID      string `json:"route_id"`
	RouteName    string `json:"route_name"`
	RouteColor   string `json:"route_color"`
	DirectionID  int    `json:"direction_id"`
	StopsBetween int    `json:"stops_between"`
	DurationSec  int    `json:"duration_sec"`
	ShapeID      string `json:"shape_id"`
	FromStop     Stop     `json:"from_stop"`
	ToStop       Stop     `json:"to_stop"`
	Stops        []string `json:"stops"`
}

type RoutePlanRoute struct {
	RouteID      string     `json:"route_id,omitempty"`
	RouteName    string     `json:"route_name,omitempty"`
	RouteColor   string     `json:"route_color,omitempty"`
	DirectionID  int        `json:"direction_id,omitempty"`
	StopsBetween int        `json:"stops_between,omitempty"`
	DurationSec  int        `json:"duration_sec,omitempty"`
	ShapeID      string     `json:"shape_id,omitempty"`
	Legs         []RouteLeg `json:"legs"`
	TransferAt   *Stop      `json:"transfer_at,omitempty"`
}

type RoutePlanResult struct {
	Routes   []RoutePlanRoute `json:"routes"`
	FromStop Stop             `json:"from_stop"`
	ToStop   Stop             `json:"to_stop"`
}

// ponytail: simple time parsing, NOT using time.Parse to avoid layout hell
func parseGTFSTime(s string) time.Duration {
	if s == "" {
		return 0
	}
	var h, m, sec int
	n, _ := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec)
	if n < 3 {
		return 0
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}

func nowDuration() time.Duration {
	t := time.Now()
	return time.Duration(t.Hour())*time.Hour +
		time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second
}
