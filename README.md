# RightmoveAlerts
## Setup

Create two lambdas on AWS (scraper, analyser).

Compile and upload both lambda functions by repeating the following for both lambdas:

- ``cd`` into lambda folder.
- Compile with ``GOARCH=amd64 GOOS=linux go build -o main *.go ``
- Compress the resultant main executable to a .zip file.
- Go onto the relevant AWS Lambda function and upload this .zip as the code.

Create an SQS queue to store the scraped ids.

Create a DynamoDB table to store your Rightmove searches and associated data.

In the table create a new item with the following attributes. You can create any number of items for different sets of searches with different parameters and different Discord webhooks.

- Id: Number. Set this as anything you want.
- Destinations: List. Add a String of each destination you want to check commute times to.
- DiscordWebhook: String. Set this to your Discord webhook URL.
- GPTPrompt: String. The listing content is appended to this prompt and fed to gpt-3.5-turbo in conversation mode. The response from GPT is added to the message sent to discord. Can be left blank.
- MaxCommuteTimeSeconds: Number. Maximum commute time via tube to any destination. Any listing exceeding this limit are not sent to Discord. Required with destinations.
- RightmoveSearchURLs: List. Add a String for each Rightmove search you want the functions to automate. Just make a search on Rightmove and copy the URL.

Now configure the lambdas.

For both lambdas set the following environment variables.
- Set SEARCH_TABLE to the name of the DynamoDB table you created
- Set SQS_QUEUE_URL to the URL of the SQS queue you created

For the analyse lambda set the following environment variables.
- Set GOOGLE_MAPS_API_KEY to your Google Map API Key (make sure to enable Distance Matrix API)
- Set GPT_API_KEY to your OpenAPI Key

Finally set up an Eventbridge rule to trigger the scraping lambda periodically for each item you want.

Configure it to deliver a constant input the lambda with the following format.

``{
"SearchId": <the id of the dynamodb item you created earlier>
}``


