package collector_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"github.com/weisshorn-cyd/gocti"

	"github.com/weisshorn-cyd/opencti_exporter/collector"
)

var errNoMockResponseLeft = errors.New("no more mocked response")

// MockTransport is a RoundTripper that returns multiple responses.
type MockTransport struct {
	Responses []*http.Response
	Index     int
}

func (m *MockTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	if m.Index >= len(m.Responses) {
		return nil, errNoMockResponseLeft
	}

	response := m.Responses[m.Index]
	m.Index++

	return response, nil
}

func TestOpenCTICollector_Collect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		transport *MockTransport
		expected  string
	}{
		{
			name: "opencti not up",
			transport: &MockTransport{
				Responses: []*http.Response{
					{
						StatusCode: http.StatusBadGateway,
						Header:     make(http.Header),
						Body:       io.NopCloser(nil),
					},
				},
			},
			expected: `
			# HELP opencti_up Wether OpenCTI is up.
			# TYPE opencti_up gauge
			opencti_up 0
			`,
		},
		{
			name: "opencti up, but no creation or update",
			transport: &MockTransport{
				Responses: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
					},
				},
			},
			expected: `
			# HELP opencti_up Wether OpenCTI is up.
			# TYPE opencti_up gauge
			opencti_up 0
			`,
		},
		{
			name: "opencti up with creation and update",
			transport: &MockTransport{
				Responses: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
					},
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"__typename":"EmailAddr","id":"585cf60b-bdc3-45c5-a909-a9dcd0434db7","standard_id":"email-addr--78b7af49-a1ce-5776-90fd-e6dd8629ec61","entity_type":"Email-Addr","observable_value":"test@test.com","created_at":"2025-01-16T15:45:55.316Z","updated_at":"2025-01-16T15:45:55.316Z","objectMarking":[],"__isStixCyberObservable":"EmailAddr","parent_types":["Basic-Object","Stix-Object","Stix-Core-Object","Stix-Cyber-Observable"],"draftVersion":null,"createdBy":null,"objectLabel":[],"creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]},"cursor":"WzE3MzcwNDIzNTUzMTYsImVtYWlsLWFkZHItLTc4YjdhZjQ5LWExY2UtNTc3Ni05MGZkLWU2ZGQ4NjI5ZWM2MSJd"}],"pageInfo":{"endCursor":"WzE3MzcwNDIzNTUzMTYsImVtYWlsLWFkZHItLTc4YjdhZjQ5LWExY2UtNTc3Ni05MGZkLWU2ZGQ4NjI5ZWM2MSJd","hasNextPage":false,"globalCount":1}}}}`)),
					},
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"__typename":"Hostname","id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","standard_id":"hostname--67400f9d-c37e-5a42-9371-029e0c459598","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-01-15T16:17:05.211Z","updated_at":"2025-01-16T15:47:03.324Z","objectMarking":[],"__isStixCyberObservable":"Hostname","parent_types":["Basic-Object","Stix-Object","Stix-Core-Object","Stix-Cyber-Observable"],"draftVersion":null,"createdBy":null,"objectLabel":[],"creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]},"cursor":"WzE3MzY5NTc4MjUyMTEsImhvc3RuYW1lLS02NzQwMGY5ZC1jMzdlLTVhNDItOTM3MS0wMjllMGM0NTk1OTgiXQ=="}],"pageInfo":{"endCursor":"WzE3MzY5NTc4MjUyMTEsImhvc3RuYW1lLS02NzQwMGY5ZC1jMzdlLTVhNDItOTM3MS0wMjllMGM0NTk1OTgiXQ==","hasNextPage":false,"globalCount":1}}}}`)),
					},
				},
			},
			expected: `
			# HELP opencti_last_created_timestamp_seconds Timestamp of the last creation in OpenCTI by entity type.
			# TYPE opencti_last_created_timestamp_seconds gauge
			opencti_last_created_timestamp_seconds{entity_type="Email-Addr"} 1.737042355e+09
			# HELP opencti_last_updated_timestamp_seconds Timestamp of the last update in OpenCTI by entity type.
			# TYPE opencti_last_updated_timestamp_seconds gauge
			opencti_last_updated_timestamp_seconds{entity_type="Hostname"} 1.737042423e+09
			# HELP opencti_up Wether OpenCTI is up.
			# TYPE opencti_up gauge
			opencti_up 1
			`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			opencti, err := gocti.NewOpenCTIAPIClient(
				"https://opencti:8080", "testtoken",
				gocti.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))),
				gocti.WithTransport(test.transport),
			)
			if err != nil {
				t.Fatalf("cannot create OpenCTI client: %v", err)
			}

			oc := collector.NewOpenCTICollector(context.Background(), opencti, "", slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

			err = testutil.CollectAndCompare(oc, strings.NewReader(test.expected), "opencti_up", "opencti_last_created_timestamp_seconds", "opencti_last_updated_timestamp_seconds")
			require.NoError(t, err)
		})
	}
}
