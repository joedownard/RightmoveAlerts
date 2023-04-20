package main

import (
	"context"
	"fmt"
	"googlemaps.github.io/maps"
	"log"
	"os"
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
	}

	rCycling := &maps.DistanceMatrixRequest{
		Origins:      []string{fmt.Sprintf("%f,%f", lat, long)},
		Destinations: destinations,
		Mode:         maps.TravelModeBicycling,
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
