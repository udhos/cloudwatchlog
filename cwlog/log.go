// Package cwlog explicitly sends logs do AWS CloudWatch.
package cwlog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// Options define settings.
type Options struct {
	// AwsConfig is required.
	// AwsConfig can be created with config.LoadDefaultConfig() from importing "github.com/aws/aws-sdk-go-v2/config".
	AwsConfig aws.Config

	// LogGroup is required.
	LogGroup string

	// LogGroupClass is optional log group class.
	// If undefined, defaults to types.LogGroupClassStandard ("STANDARD").
	LogGroupClass types.LogGroupClass

	// LogStream defaults to LogGroup.
	LogStream string

	// LogStream defaults to "{{.LogStream}}-{{.YYYY}}-{{.MM}}-{{.DD}}-{{.HH}}"
	LogStreamTemplate string

	// RetentionInDays defaults to 30.
	RetentionInDays int32

	// Client optionally provides CloudWatch Logs client, for testing.
	// If undefined, it is created automatically from AwsConfig.
	Client CloudWatchLogClient

	// Now is optional function to get current time, for testing.
	// If undefined, defaults to time.Time().
	Now func() time.Time
}

var defaultStreamTemplate = "{{.LogStream}}-{{.YYYY}}-{{.MM}}-{{.DD}}-{{.HH}}"

// Log holds cloudwatch client context.
type Log struct {
	options       Options
	logStreamName string // last used log stream name
	templ         *template.Template
}

// New creates cloudwatch client context.
func New(options Options) (*Log, error) {

	if options.LogGroup == "" {
		return nil, errors.New("LogGroup is required")
	}

	if options.LogStream == "" {
		options.LogStream = options.LogGroup
	}

	if options.LogStreamTemplate == "" {
		options.LogStreamTemplate = defaultStreamTemplate
	}

	tmpl, errTemplate := template.New("logStream").Parse(options.LogStreamTemplate)
	if errTemplate != nil {
		return nil, fmt.Errorf("log stream template error: %v", errTemplate)
	}

	if options.RetentionInDays == 0 {
		options.RetentionInDays = 30
	}

	if options.Client == nil {
		options.Client = cloudwatchlogs.NewFromConfig(options.AwsConfig)
	}

	if options.Now == nil {
		options.Now = time.Now
	}

	groupInput := &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName:  aws.String(options.LogGroup),
		LogGroupClass: options.LogGroupClass,
	}

	if _, errCreateGroup := options.Client.CreateLogGroup(context.TODO(),
		groupInput); errCreateGroup != nil {

		var errExists *types.ResourceAlreadyExistsException
		if !errors.As(errCreateGroup, &errExists) {
			// other error than "already exists" must be reported
			return nil, fmt.Errorf("create group error: %s: %v", options.LogGroup, errCreateGroup)
		}

		// here: already exists error is benign
	}
	if _, errRetention := options.Client.PutRetentionPolicy(context.TODO(),
		&cloudwatchlogs.PutRetentionPolicyInput{LogGroupName: aws.String(options.LogGroup),
			RetentionInDays: aws.Int32(options.RetentionInDays)}); errRetention != nil {
		return nil, fmt.Errorf("put group retention error: group=%s retention=%d: %v",
			options.LogGroup, options.RetentionInDays, errRetention)
	}

	cw := &Log{
		options: options,
		templ:   tmpl,
	}
	return cw, nil
}

// LogStreamFields defines fields for log stream name.
type LogStreamFields struct {
	LogGroup  string
	LogStream string
	YYYY      string
	MM        string
	DD        string
	HH        string
}

func genStream(templ *template.Template, group, stream string, now time.Time) (string, error) {
	fields := LogStreamFields{
		LogGroup:  group,
		LogStream: stream,
		YYYY:      now.Format("2006"),
		MM:        now.Format("01"),
		DD:        now.Format("02"),
		HH:        now.Format("15"),
	}
	var buf bytes.Buffer
	err := templ.Execute(&buf, fields)
	return buf.String(), err
}

func (l *Log) generateStreamName() string {
	stream, err := genStream(l.templ, l.options.LogGroup, l.options.LogStream, l.options.Now())
	if err != nil {
		panic(err) // ugh
	}
	return stream
}

// PutSimple sends a simple log line.
func (l *Log) PutSimple(s string) error {
	now := l.options.Now().UnixMilli()
	return l.PutLogEvents([]types.InputLogEvent{
		{
			Message:   aws.String(s),
			Timestamp: aws.Int64(now),
		},
	})
}

// PutLogEvents sends logs.
func (l *Log) PutLogEvents(events []types.InputLogEvent) error {

	logStream := l.generateStreamName()

	if logStream != l.logStreamName {
		//
		// log stream has changed, create it
		//
		if _, errCreateStream := l.options.Client.CreateLogStream(context.TODO(),
			&cloudwatchlogs.CreateLogStreamInput{LogGroupName: aws.String(l.options.LogGroup),
				LogStreamName: aws.String(logStream)}); errCreateStream != nil {

			var errExists *types.ResourceAlreadyExistsException
			if !errors.As(errCreateStream, &errExists) {
				// other error than "already exists" must be reported

				l.logStreamName = "" // empty will force new attempt

				return fmt.Errorf("create log stream error: group=%s stream=%s: %v",
					l.options.LogGroup, logStream, errCreateStream)
			}

			// here: already exists error is benign
		} else {
			//
			// created, update log stream
			//
			l.logStreamName = logStream
		}
	}

	input := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     events,
		LogGroupName:  aws.String(l.options.LogGroup),
		LogStreamName: aws.String(logStream),
	}

	_, errPut := l.options.Client.PutLogEvents(context.TODO(), input)
	if errPut != nil {
		return fmt.Errorf("PutLogEvents error: group=%s stream=%s: %v",
			l.options.LogGroup, logStream, errPut)
	}

	return nil
}

// CloudWatchLogClient defines testable interface for plugging in CloudWatch Logs client.
type CloudWatchLogClient interface {
	CreateLogGroup(ctx context.Context,
		params *cloudwatchlogs.CreateLogGroupInput,
		optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	PutRetentionPolicy(ctx context.Context,
		params *cloudwatchlogs.PutRetentionPolicyInput,
		optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	CreateLogStream(ctx context.Context,
		params *cloudwatchlogs.CreateLogStreamInput,
		optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	PutLogEvents(ctx context.Context,
		params *cloudwatchlogs.PutLogEventsInput,
		optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
}
