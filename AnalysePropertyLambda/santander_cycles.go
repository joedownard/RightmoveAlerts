package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type CyclesHub struct {
	StationName string `json:"stationName"`
	Location    struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
}

type CyclesGQLResponse struct {
	Data struct {
		Supply struct {
			Stations []CyclesHub `json:"stations"`
		} `json:"supply"`
	} `json:"data"`
}

func GetClosestHubLatLng(lat, lng float64) (CyclesHub, error) {
	dat, err := os.ReadFile("./cyclehubs.json")
	if err != nil {
		fmt.Printf("Failed to load cyclehubs json data, here's why: %v", err)
		return CyclesHub{}, err
	}

	var cycleHubsData CyclesGQLResponse
	err = json.Unmarshal(dat, &cycleHubsData)
	if err != nil {
		fmt.Printf("Failed to unmarshal cyclehubs json data, here's why: %v", err)
		return CyclesHub{}, err
	}

	stations := cycleHubsData.Data.Supply.Stations

	closest := math.MaxFloat64
	closestIdx := 0
	for i, station := range stations {
		sLat, sLng := station.Location.Lat, station.Location.Lng
		absDistance := math.Pow(sLat-lat, 2) + math.Pow(sLng-lng, 2)
		if absDistance < closest {
			closest = absDistance
			closestIdx = i
		}
	}

	return stations[closestIdx], nil
}
