// Package main implements the example.
package main

import (
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/udhos/boilerplate/awsconfig"
	"github.com/udhos/cloudwatchlog/cwlog"
)

func main() {
	awsConfig, errAwsConfig := awsconfig.AwsConfig(awsconfig.Options{})
	if errAwsConfig != nil {
		log.Fatalf("aws sdk config error: %v", errAwsConfig)
	}
	cw, err := cwlog.New(cwlog.Options{
		AwsConfig: awsConfig.AwsConfig,
		LogGroup:  "/cloudwatchlogs/example",
		LogStream: "/cloudwatchlogs/example",
	})
	if err != nil {
		log.Fatalf("client error: %v", err)
	}
	now := time.Now().UnixMilli()
	events := []types.InputLogEvent{
		{
			Message:   aws.String("hello cloudwatchlog-example - 1"),
			Timestamp: aws.Int64(now),
		},
		{
			Message:   aws.String("hello cloudwatchlog-example - 2"),
			Timestamp: aws.Int64(now),
		},
	}
	if err := cw.PutLogEvents(events); err != nil {
		log.Printf("log failed: %v", err)
	}
}
