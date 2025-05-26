package cwlog

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

type streamTest struct {
	name           string
	group          string
	stream         string
	streamTemplate string
	now            time.Time
	expected       string
}

var streamTestTable = []streamTest{
	{
		name:           "stream empty",
		stream:         "stream1",
		streamTemplate: defaultStreamTemplate,
		now:            time.Time{},
		expected:       "stream1-0001-01-01-00",
	},
}

func TestStreamName(t *testing.T) {
	for i, data := range streamTestTable {
		name := fmt.Sprintf("%02d of %02d: %s", i+1, len(streamTestTable), data.name)

		tmpl, errTemplate := template.New("logStream").Parse(data.streamTemplate)
		if errTemplate != nil {
			t.Fatalf("%s: template error: %v", name, errTemplate)
		}

		stream, errStream := genStream(tmpl, data.group, data.stream, data.now)
		if errStream != nil {
			t.Fatalf("%s: generate stream error: %v", name, errStream)
		}

		if stream != data.expected {
			t.Fatalf("%s: stream: expected=%s got=%s", name, data.expected, stream)
		}
	}
}

func TestSendSimple(t *testing.T) {
	client := newCloudWatchLogMock()
	cw, err := New(Options{
		Client:    client,
		Now:       func() time.Time { return time.Time{} },
		LogGroup:  "/cloudwatchlogs/group",
		LogStream: "/cloudwatchlogs/stream",
	})
	if err != nil {
		t.Fatal(err)
	}
	g, found := client.groups["/cloudwatchlogs/group"]
	if !found {
		t.Fatalf("log group not found")
	}
	if err := cw.PutSimple("test 1"); err != nil {
		t.Fatal(err)
	}
	if err := cw.PutSimple("test 2"); err != nil {
		t.Fatal(err)
	}
	s, foundStream := g["/cloudwatchlogs/stream-0001-01-01-00"]
	if !foundStream {
		t.Fatal("stream not found")
	}
	if len(s) != 2 {
		t.Fatalf("log lines: expected=2 found=%d", len(s))
	}
}

func TestChangeStream(t *testing.T) {
	var now time.Time
	client := newCloudWatchLogMock()
	cw, err := New(Options{
		Client:    client,
		Now:       func() time.Time { return now },
		LogGroup:  "/cloudwatchlogs/group",
		LogStream: "/cloudwatchlogs/stream",
	})
	if err != nil {
		t.Fatal(err)
	}
	g, found := client.groups["/cloudwatchlogs/group"]
	if !found {
		t.Fatalf("log group not found")
	}
	if err := cw.PutSimple("test 1"); err != nil {
		t.Fatal(err)
	}
	if err := cw.PutSimple("test 2"); err != nil {
		t.Fatal(err)
	}
	{
		s, foundStream := g["/cloudwatchlogs/stream-0001-01-01-00"]
		if !foundStream {
			t.Fatal("stream not found")
		}
		if len(s) != 2 {
			t.Fatalf("log lines: expected=2 found=%d", len(s))
		}
	}

	// increase one day
	now = now.Add(time.Hour * 24)

	if err := cw.PutSimple("test 1"); err != nil {
		t.Fatal(err)
	}
	s, foundStream := g["/cloudwatchlogs/stream-0001-01-02-00"]
	if !foundStream {
		t.Fatal("stream not found")
	}
	if len(s) != 1 {
		t.Fatalf("log lines: expected=1 found=%d", len(s))
	}
}

func TestGroupExists(t *testing.T) {
	client := newCloudWatchLogMock()

	//
	// first create group
	//
	{
		cw, err := New(Options{
			Client:    client,
			Now:       func() time.Time { return time.Time{} },
			LogGroup:  "/cloudwatchlogs/group",
			LogStream: "/cloudwatchlogs/stream",
		})
		if err != nil {
			t.Fatal(err)
		}
		g, found := client.groups["/cloudwatchlogs/group"]
		if !found {
			t.Fatalf("log group not found")
		}
		if err := cw.PutSimple("test 1"); err != nil {
			t.Fatal(err)
		}
		if err := cw.PutSimple("test 2"); err != nil {
			t.Fatal(err)
		}
		s, foundStream := g["/cloudwatchlogs/stream-0001-01-01-00"]
		if !foundStream {
			t.Fatal("stream not found")
		}
		if len(s) != 2 {
			t.Fatalf("log lines: expected=2 found=%d", len(s))
		}
	}

	//
	// repeat, but now the group exists
	//

	cw, err := New(Options{
		Client:    client,
		Now:       func() time.Time { return time.Time{} },
		LogGroup:  "/cloudwatchlogs/group",
		LogStream: "/cloudwatchlogs/stream",
	})
	if err != nil {
		t.Fatal(err)
	}
	g, found := client.groups["/cloudwatchlogs/group"]
	if !found {
		t.Fatalf("log group not found")
	}
	if err := cw.PutSimple("test 3"); err != nil {
		t.Fatal(err)
	}
	if err := cw.PutSimple("test 4"); err != nil {
		t.Fatal(err)
	}
	s, foundStream := g["/cloudwatchlogs/stream-0001-01-01-00"]
	if !foundStream {
		t.Fatal("stream not found")
	}
	if len(s) != 4 {
		t.Fatalf("log lines: expected=4 found=%d", len(s))
	}
}

func newCloudWatchLogMock() *cloudWatchLogMock {
	return &cloudWatchLogMock{groups: map[string]map[string][]types.InputLogEvent{}}
}

type cloudWatchLogMock struct {
	denyCreateGroup  bool
	denyRetention    bool
	denyCreateStream bool
	denyPutLog       bool
	groups           map[string]map[string][]types.InputLogEvent
	retentionInDays  int32
}

func (m *cloudWatchLogMock) CreateLogGroup(_ context.Context,
	params *cloudwatchlogs.CreateLogGroupInput,
	_ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	if m.denyCreateGroup {
		return nil, errors.New("create group denied")
	}
	groupName := aws.ToString(params.LogGroupName)
	_, foundGroup := m.groups[groupName]
	if foundGroup {
		return nil, &types.ResourceAlreadyExistsException{
			Message: aws.String("The specified log group already exists"),
		}
	}
	g := map[string][]types.InputLogEvent{}
	m.groups[groupName] = g
	return &cloudwatchlogs.CreateLogGroupOutput{}, nil
}

func (m *cloudWatchLogMock) PutRetentionPolicy(_ context.Context,
	params *cloudwatchlogs.PutRetentionPolicyInput,
	_ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	if m.denyRetention {
		return nil, errors.New("put retention denied")
	}
	m.retentionInDays = aws.ToInt32(params.RetentionInDays)
	return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
}

func (m *cloudWatchLogMock) CreateLogStream(_ context.Context,
	params *cloudwatchlogs.CreateLogStreamInput,
	_ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	if m.denyCreateStream {
		return nil, errors.New("create stream denied")
	}
	groupName := aws.ToString(params.LogGroupName)
	g, foundGroup := m.groups[groupName]
	if !foundGroup {
		return nil, fmt.Errorf("group not found: %s", groupName)
	}
	streamName := aws.ToString(params.LogStreamName)
	_, foundStream := g[streamName]
	if foundStream {
		return nil, &types.ResourceAlreadyExistsException{
			Message: aws.String("The specified log stream already exists"),
		}
	}
	s := []types.InputLogEvent{}
	g[streamName] = s
	return &cloudwatchlogs.CreateLogStreamOutput{}, nil
}

func (m *cloudWatchLogMock) PutLogEvents(_ context.Context,
	params *cloudwatchlogs.PutLogEventsInput,
	_ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	if m.denyPutLog {
		return nil, errors.New("put log denied")
	}
	groupName := aws.ToString(params.LogGroupName)
	g, foundGroup := m.groups[groupName]
	if !foundGroup {
		return nil, fmt.Errorf("group not found: %s", groupName)
	}
	streamName := aws.ToString(params.LogStreamName)
	s, foundStream := g[streamName]
	if !foundStream {
		return nil, fmt.Errorf("stream not found: group=%s stream=%s",
			groupName, streamName)
	}
	for _, e := range params.LogEvents {
		s = append(s, e)
	}
	g[streamName] = s
	return &cloudwatchlogs.PutLogEventsOutput{}, nil
}
