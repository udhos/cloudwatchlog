[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/udhos/cloudwatchlog/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/udhos/cloudwatchlog)](https://goreportcard.com/report/github.com/udhos/cloudwatchlog)
[![Go Reference](https://pkg.go.dev/badge/github.com/udhos/cloudwatchlog.svg)](https://pkg.go.dev/github.com/udhos/cloudwatchlog)

# cloudwatchlog

This Go module [https://github.com/udhos/cloudwatchlog](https://github.com/udhos/cloudwatchlog) helps in explicitly sending log events do AWS CloudWatch.

# Synopsis

```golang
import "github.com/udhos/cloudwatchlog/cwlog"

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
cw.PutLogEvents(events)
```

# Example

See [./examples/cloudwatchlog-example/main.go](./examples/cloudwatchlog-example/main.go).
