package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
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

type RightmoveAPIResponse struct {
	Properties []PropertyIdWithLocation `json:"properties"`

	Pagination struct {
		Next *string `json:"next"`
	} `json:"pagination"`
}

type PropertyIdWithLocation struct {
	Id       int64 `json:"Id"`
	Location struct {
		Latitude  float32 `json:"Latitude"`
		Longitude float32 `json:"Longitude"`
	} `json:"Location"`
}

type SqsMessage struct {
	SearchId        int64                  `json:"SearchId"`
	PropertyDetails PropertyIdWithLocation `json:"PropertyIdWithLocation"`
}

var (
	DynamoDbClient       *dynamodb.Client
	SqsClient            *sqs.Client
	RightmoveSearchTable = aws.String(os.Getenv("SEARCH_TABLE"))
	AwsRegion            = os.Getenv("AWS_REGION")
	SqsQueueURL          = os.Getenv("SQS_QUEUE_URL")
	RightmoveAPIBase     = "https://www.rightmove.co.uk/api/_search?"
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
	SqsClient = sqs.NewFromConfig(cfg)
}

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(event struct{ SearchId int64 }) error {
	search, _ := GetSearch(event.SearchId)
	oldIdsMap := make(map[int64]struct{})
	currentIdsWithLocMap := make(map[int64]PropertyIdWithLocation)
	newIdsWithLocMap := make(map[int64]PropertyIdWithLocation)

	currentIds := make([]int64, 0)

	for _, id := range search.PreviousPropertyIds {
		oldIdsMap[id] = struct{}{}
	}

	for _, searchURL := range search.RightmoveSearchURLs {
		fmt.Printf("Looking for ids for the following search: %s\n", searchURL)
		idsWithLocation := GetPropertyIdsWithLocationFromRightmoveSearchURL(searchURL)
		for _, idWithLoc := range idsWithLocation {
			currentIdsWithLocMap[idWithLoc.Id] = idWithLoc
		}
	}

	for id, idWithLoc := range currentIdsWithLocMap {
		currentIds = append(currentIds, id)
		if _, dup := oldIdsMap[id]; !dup {
			fmt.Printf("New id identified: %d\n", id)
			newIdsWithLocMap[id] = idWithLoc
		}
	}

	search.PreviousPropertyIds = currentIds
	UpdateSearch(search)

	for _, idWithLoc := range newIdsWithLocMap {
		message := SqsMessage{
			SearchId:        search.Id,
			PropertyDetails: idWithLoc,
		}

		messageJson, err := json.Marshal(message)
		if err != nil {
			fmt.Printf("Encountered an error while marshalling the message: %v\n", err)
			return err
		}

		messageInput := &sqs.SendMessageInput{
			MessageBody: aws.String(string(messageJson)),
			QueueUrl:    &SqsQueueURL,
		}

		_, err = SqsClient.SendMessage(context.TODO(), messageInput)
		if err != nil {
			fmt.Printf("Encountered an error while sending the message: %v\n", err)
			return err
		}
	}

	return nil
}

func UpdateSearch(search RightmoveSearch) {
	item, err := attributevalue.MarshalMap(search)
	if err != nil {
		log.Printf("Couldn't marshal updated info. Here's why: %v\n", err)
	}

	_, err = DynamoDbClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: RightmoveSearchTable,
		Item:      item,
	})
	if err != nil {
		log.Printf("Couldn't add item to the searches table. Here's why: %v\n", err)
	}
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

func GetPropertyIdsWithLocationFromRightmoveSearchURL(searchURL string) []PropertyIdWithLocation {
	s, err := url.Parse(searchURL)
	if err != nil {
		fmt.Println("Failed to parse search URL")
		return nil
	}

	values := s.Query()
	values.Del("dontShow")
	values.Del("mustHave")
	values.Del("furnishTypes")
	values.Add("sortType", "6")
	values.Add("index", "0")
	values.Add("viewType", "LIST")
	values.Add("channel", "RENT")
	values.Add("areaSizeUnit", "sqft")
	values.Add("currencyCode", "GBP")
	values.Add("isFetching", "false")

	startIndex := "0"
	resp := RightmoveAPIResponse{
		Pagination: struct {
			Next *string `json:"next"`
		}{Next: &startIndex},
	}

	var idsWithLoc []PropertyIdWithLocation

	for resp.Pagination.Next != nil {
		values.Set("index", *resp.Pagination.Next)

		query := RightmoveAPIBase + values.Encode()
		raw := RunQuery(query)

		resp = RightmoveAPIResponse{}
		err = json.Unmarshal(raw, &resp)
		if err != nil {
			fmt.Println(err)
		}

		for _, p := range resp.Properties {
			idsWithLoc = append(idsWithLoc, p)
		}
	}

	return idsWithLoc
}

func RunQuery(query string) []byte {
	client := http.Client{
		Timeout: time.Second * 5,
	}
	req, err := http.NewRequest(http.MethodGet, query, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36")

	res, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	return body
}
