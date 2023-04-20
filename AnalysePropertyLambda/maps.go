package main

import (
	"context"
	"fmt"
	"googlemaps.github.io/maps"
	"log"
	"os"
	"strconv"
	"time"
)

type CommuteTime struct {
	Destination        string
	TimeMinutesTube    int64
	TimeMinutesCycling int64
}

var (
	GoogleMapsApiKey = os.Getenv("GOOGLE_MAPS_API_KEY")
)

func GetCommuteTimes(destinations []string, lat, long float32) []CommuteTime {
	fmt.Printf("Checking commute times to location with lat: %f, long: %f\n", lat, long)

	c, err := maps.NewClient(maps.WithAPIKey(GoogleMapsApiKey))
	if err != nil {
		log.Println(err)
		return nil
	}

	rTransport := &maps.DistanceMatrixRequest{
		Origins:      []string{fmt.Sprintf("%f,%f", lat, long)},
		Destinations: destinations,
		Mode:         maps.TravelModeTransit,
		TransitMode:  []maps.TransitMode{maps.TransitModeRail},
		ArrivalTime:  GetClosestHourOnWeekday(9, 1),
	}

	rCycling := &maps.DistanceMatrixRequest{
		Origins:      []string{fmt.Sprintf("%f,%f", lat, long)},
		Destinations: destinations,
		Mode:         maps.TravelModeBicycling,
		ArrivalTime:  GetClosestHourOnWeekday(9, 1),
	}

	respTransport, err := c.DistanceMatrix(context.Background(), rTransport)
	if err != nil {
		log.Println(err)
		return nil
	}

	respCycling, err := c.DistanceMatrix(context.Background(), rCycling)
	if err != nil {
		log.Println(err)
		return nil
	}

	tubeResults := respTransport.Rows[0].Elements
	cycleResults := respCycling.Rows[0].Elements

	commuteTimes := make([]CommuteTime, 0)
	for i, dest := range destinations {
		commuteTime := CommuteTime{
			Destination:        dest,
			TimeMinutesTube:    int64(tubeResults[i].Duration.Minutes()),
			TimeMinutesCycling: int64(cycleResults[i].Duration.Minutes()),
		}
		commuteTimes = append(commuteTimes, commuteTime)
	}

	return commuteTimes
}

func GetClosestHourOnWeekday(hour, weekday int64) string {
	now := time.Now().Round(time.Hour)
	daysToMonday := int64(now.Weekday()) - weekday
	hoursTo9Am := int64(now.Hour()) - hour

	now = now.Add(time.Hour * 24 * time.Duration(daysToMonday))
	now = now.Add(time.Hour * time.Duration(hoursTo9Am))
	return strconv.FormatInt(now.Unix(), 10)
}
