package main

import (
	"fmt"
	"strconv"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/fatih/color"
	"github.com/tj/go-progress"
	"github.com/tj/go/term"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	sourceQueue      = kingpin.Flag("source", "Source queue to move messages from").Short('s').Required().String()
	destinationQueue = kingpin.Flag("destination", "Destination queue to move messages to").Short('d').Required().String()
	profile          = kingpin.Flag("profile", "AWS Profile for source and destination queues").Short('p').Default("default").String()
	region           = kingpin.Flag("region", "AWS Region for source and destination queues").Short('r').Default("us-east-1").String()
)

func main() {
	log.SetHandler(cli.Default)

	fmt.Println()
	defer fmt.Println()

	kingpin.UsageTemplate(kingpin.CompactUsageTemplate)
	kingpin.Parse()

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: *profile,
		Config: aws.Config{
			Region: aws.String(*region),
		},
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		log.Error(color.New(color.FgRed).Sprintf("Unable to create AWS session for region \r\n", *region))
		return
	}

	svc := sqs.New(sess)

	sourceQueueURL, err := resolveQueueURL(svc, *sourceQueue)

	if err != nil {
		logAwsError("Failed to resolve source queue", err)
		return
	}

	log.Info(color.New(color.FgCyan).Sprintf("Source queue URL: %s", sourceQueueURL))

	destinationQueueURL, err := resolveQueueURL(svc, *destinationQueue)

	if err != nil {
		logAwsError("Failed to resolve destination queue", err)
		return
	}

	log.Info(color.New(color.FgCyan).Sprintf("Destination queue URL: %s", destinationQueueURL))

	queueAttributes, err := svc.GetQueueAttributes(&sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(sourceQueueURL),
		AttributeNames: []*string{aws.String("All")},
	})

	numberOfMessages, _ := strconv.Atoi(*queueAttributes.Attributes["ApproximateNumberOfMessages"])

	log.Info(color.New(color.FgCyan).Sprintf("Approximate number of messages in the source queue: %s",
		*queueAttributes.Attributes["ApproximateNumberOfMessages"]))

	if numberOfMessages == 0 {
		log.Info("Looks like nothing to move. Done.")
		return
	}

	moveMessages(sourceQueueURL, destinationQueueURL, svc, numberOfMessages)

}

func resolveQueueURL(svc *sqs.SQS, queueName string) (string, error) {
	params := &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	}
	resp, err := svc.GetQueueUrl(params)

	if err != nil {
		return "", err
	}

	return *resp.QueueUrl, nil
}

func logAwsError(message string, err error) {
	if awsErr, ok := err.(awserr.Error); ok {
		log.Error(color.New(color.FgRed).Sprintf("%s. Error: %s", message, awsErr.Message()))
	} else {
		log.Error(color.New(color.FgRed).Sprintf("%s. Error: %s", message, err.Error()))
	}
}

func convertToEntries(messages []*sqs.Message) []*sqs.SendMessageBatchRequestEntry {
	result := make([]*sqs.SendMessageBatchRequestEntry, len(messages))
	for i, message := range messages {
		result[i] = &sqs.SendMessageBatchRequestEntry{
			MessageBody: message.Body,
			Id:          message.MessageId,
		}
	}

	return result
}

func convertSuccessfulMessageToBatchRequestEntry(messages []*sqs.Message) []*sqs.DeleteMessageBatchRequestEntry {
	result := make([]*sqs.DeleteMessageBatchRequestEntry, len(messages))
	for i, message := range messages {
		result[i] = &sqs.DeleteMessageBatchRequestEntry{
			ReceiptHandle: message.ReceiptHandle,
			Id:            message.MessageId,
		}
	}

	return result
}

func moveMessages(sourceQueueURL string, destinationQueueURL string, svc *sqs.SQS, numberOfMessages int) {
	params := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(sourceQueueURL),
		VisibilityTimeout:   aws.Int64(2),
		WaitTimeSeconds:     aws.Int64(0),
		MaxNumberOfMessages: aws.Int64(10),
	}

	log.Info(color.New(color.FgCyan).Sprintf("Starting to move messages..."))
	fmt.Println()

	term.HideCursor()
	defer term.ShowCursor()

	b := progress.NewInt(numberOfMessages)
	b.Width = 40
	b.StartDelimiter = color.New(color.FgCyan).Sprint("|")
	b.EndDelimiter = color.New(color.FgCyan).Sprint("|")
	b.Filled = color.New(color.FgCyan).Sprint("█")
	b.Empty = color.New(color.FgCyan).Sprint("░")
	b.Template(`		{{.Bar}} {{.Text}}{{.Percent | printf "%3.0f"}}%`)

	render := term.Renderer()

	messagesProcessed := 0

	for {
		resp, err := svc.ReceiveMessage(params)

		if len(resp.Messages) == 0 {
			fmt.Println()
			log.Info(color.New(color.FgCyan).Sprintf("Done. Moved %s messages", strconv.Itoa(numberOfMessages)))
			return
		}

		if err != nil {
			logAwsError("Failed to receive messages", err)
			return
		}

		batch := &sqs.SendMessageBatchInput{
			QueueUrl: aws.String(destinationQueueURL),
			Entries:  convertToEntries(resp.Messages),
		}

		sendResp, err := svc.SendMessageBatch(batch)

		if err != nil {
			logAwsError("Failed to un-queue messages to the destination", err)
			return
		}

		if len(sendResp.Failed) > 0 {
			log.Error(color.New(color.FgRed).Sprintf("%s messages failed to enqueue, exiting", len(sendResp.Failed)))
			return
		}

		if len(sendResp.Successful) == len(resp.Messages) {
			deleteMessageBatch := &sqs.DeleteMessageBatchInput{
				Entries:  convertSuccessfulMessageToBatchRequestEntry(resp.Messages),
				QueueUrl: aws.String(sourceQueueURL),
			}

			deleteResp, err := svc.DeleteMessageBatch(deleteMessageBatch)

			if err != nil {
				logAwsError("Failed to delete messages from source queue", err)
				return
			}

			if len(deleteResp.Failed) > 0 {
				log.Error(color.New(color.FgRed).Sprintf("Error deleting messages, the following were not deleted\n %s", deleteResp.Failed))
				return
			}

			messagesProcessed += len(resp.Messages)
		}

		// Increase the total if the approximation was under - avoids exception
		if messagesProcessed > numberOfMessages {
			b.Total = float64(messagesProcessed)
		}

		b.ValueInt(messagesProcessed)
		render(b.String())
	}
}
