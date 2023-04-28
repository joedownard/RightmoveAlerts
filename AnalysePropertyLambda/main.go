package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type RightmoveSearch struct {
	Id                    int64    `dynamodbav:"Id"`
	RightmoveSearchURLs   []string `dynamodbav:"RightmoveSearchURLs"`
	PreviousPropertyIds   []int64  `dynamodbav:"PreviousPropertyIds"`
	DiscordWebhook        string   `dynamodbav:"DiscordWebhook"`
	Destinations          []string `dynamodbav:"Destinations"`
	GPTPrompt             string   `dynamodbav:"GPTPrompt"`
	MaxCommuteTimeSeconds int64    `dynamodbav:"MaxCommuteTimeSeconds"`
}

type RightmoveListingDetails struct {
	Title       string
	Link        string
	Rent        string
	Description string
	ImageURL    string
}

type DiscordEmbed struct {
	Title       string        `json:"title"`
	Url         string        `json:"url"`
	Description string        `json:"description"`
	Image       ImageResource `json:"image"`
}

type WebhookRequest struct {
	Embeds []DiscordEmbed `json:"embeds"`
}

type ImageResource struct {
	URL *string `json:"url,omitempty"`
}

type PropertyIdWithLocation struct {
	Id       int64 `json:"Id"`
	Location struct {
		Latitude  float64 `json:"Latitude"`
		Longitude float64 `json:"Longitude"`
	} `json:"Location"`
}

type Event struct {
	SearchId        int64                  `json:"SearchId"`
	PropertyDetails PropertyIdWithLocation `json:"PropertyIdWithLocation"`
}

var (
	DynamoDbClient       *dynamodb.Client
	RightmoveSearchTable = aws.String(os.Getenv("SEARCH_TABLE"))
	AwsRegion            = os.Getenv("AWS_REGION")
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO(), func(o *config.LoadOptions) error {
		o.Region = AwsRegion
		return nil
	})
	if err != nil {
		log.Printf("Couldn't marshal updated info. Here's why: %v\n", err)
		panic(err)
	}
	DynamoDbClient = dynamodb.NewFromConfig(cfg)
}

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(sqsEvent events.SQSEvent) error {
	for _, message := range sqsEvent.Records {
		event := Event{}
		err := json.Unmarshal([]byte(message.Body), &event)

		lat, lng := event.PropertyDetails.Location.Latitude, event.PropertyDetails.Location.Longitude

		fmt.Printf("Got event with search id: %d\n", event.SearchId)

		search, err := GetSearch(event.SearchId)
		if err != nil {
			fmt.Printf("Failed to get related search info, here's why: %v\n", err)
			continue
		}

		fmt.Printf("Checking property with id: %d\n", event.PropertyDetails.Id)
		listingDetails, err := GetListingDetailsFromPropertyId(event.PropertyDetails.Id)

		if err != nil {
			message := WebhookRequest{
				Embeds: []DiscordEmbed{
					{Title: "Scraping Error", Url: "https://www.rightmove.co.uk/properties/" + strconv.FormatInt(int64(event.PropertyDetails.Id), 10), Description: "Failed to get property details, it may be invalid. Please check manually."},
				},
			}
			FireWebhook(search.DiscordWebhook, message)
			continue
		}

		gptResponse := ""
		if search.GPTPrompt != "" {
			gptResponse = RunGPTPromptWithDescription(listingDetails.Description, search.GPTPrompt)
		}

		var description strings.Builder
		description.WriteString("**Rent: " + listingDetails.Rent + "**\n")
		if search.GPTPrompt != "" {
			description.WriteRune('\n')
			description.WriteString(gptResponse)
			description.WriteRune('\n')
		}

		commuteTimes := GetCommuteTimes(search.Destinations, lat, lng)
		overMaxCommute := false
		for _, t := range commuteTimes {
			if (t.TimeMinutesTube * 60) > search.MaxCommuteTimeSeconds {
				fmt.Printf("Tube commute of %d seconds is longer than max commute time of %d seconds.\n", t.TimeMinutesTube*60, search.MaxCommuteTimeSeconds)
				overMaxCommute = true
			}
			description.WriteString(fmt.Sprintf("\n%s: 🚇 %dm   🚴 %dm", t.Destination, t.TimeMinutesTube, t.TimeMinutesCycling))
		}
		if overMaxCommute {
			fmt.Println("Commute too long, skipping property.")
			continue
		}

		closestHub, err := GetClosestHubLatLng(lat, lng)
		if err == nil {
			timeToCyclingHub := GetWalkingTime(lat, lng, closestHub.Location.Lat, closestHub.Location.Lng)
			description.WriteString(fmt.Sprintf("\n🚴 Hub (%s): %dm", closestHub.StationName, int(timeToCyclingHub)))
		}

		message := WebhookRequest{
			Embeds: []DiscordEmbed{
				{
					Title:       listingDetails.Title,
					Url:         listingDetails.Link,
					Description: description.String(),
					Image:       ImageResource{URL: &listingDetails.ImageURL},
				},
			},
		}
		FireWebhook(search.DiscordWebhook, message)
	}
	return nil
}

func GetSearch(id int64) (RightmoveSearch, error) {
	params := dynamodb.GetItemInput{
		Key: map[string]types.AttributeValue{
			"Id": &types.AttributeValueMemberN{
				Value: strconv.FormatInt(id, 10),
			},
		},
		TableName: RightmoveSearchTable,
	}

	result, err := DynamoDbClient.GetItem(context.TODO(), &params)
	if err != nil {
		log.Println("Error getting item,", err)
		return RightmoveSearch{}, err
	}

	if result.Item == nil {
		log.Println("Nothing here yet!", err)
		return RightmoveSearch{}, err
	}

	var search RightmoveSearch

	err = attributevalue.UnmarshalMap(result.Item, &search)
	if err != nil {
		log.Println("Error unmarshalling item,", err)
		return RightmoveSearch{}, err
	}

	return search, nil
}

func FireWebhook(webhook string, message WebhookRequest) {
	fmt.Println("Firing webhook")
	reqJson, err := json.Marshal(message)
	if err != nil {
		fmt.Println(err)
	}

	request, err := http.NewRequest(http.MethodPost, webhook, bytes.NewBuffer(reqJson))
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		fmt.Printf("Failed to fire webhook, here's why: %v\n", err)
	}
}
