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
		name         string
		transport    *MockTransport
		entityTypes  []string
		creatorNames []string
		expected     string
	}{
		{
			name: "no filters",
			transport: &MockTransport{
				Responses: []*http.Response{
					// Health check
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
					},
					// ("", "", created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"585cf60b-bdc3-45c5-a909-a9dcd0434db7","entity_type":"Email-Addr","observable_value":"abc@test.com","created_at":"2025-06-16T15:45:55.316Z","updated_at":"2025-06-16T15:45:55.316Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", "", updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-01-15T16:17:05.211Z","updated_at":"2025-07-13T11:29:13.100Z","creators":[{"id":"43dfeb13-95d2-4160-9002-d63a6e611aff","name":"reporter"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
				},
			},
			entityTypes:  nil,
			creatorNames: nil,
			expected: `
			# HELP opencti_last_created_timestamp_seconds Timestamp of the last creation in OpenCTI by entity type and creator.
			# TYPE opencti_last_created_timestamp_seconds gauge
			opencti_last_created_timestamp_seconds{creator="",entity_type=""} 1.750088755e+09
			# HELP opencti_last_updated_timestamp_seconds Timestamp of the last update in OpenCTI by entity type and creator.
			# TYPE opencti_last_updated_timestamp_seconds gauge
			opencti_last_updated_timestamp_seconds{creator="",entity_type=""} 1.752406153e+09
			# HELP opencti_up Wether OpenCTI is up.
			# TYPE opencti_up gauge
			opencti_up 1
			`,
		},
		{
			name: "creators only, no entity types",
			transport: &MockTransport{
				Responses: []*http.Response{
					// Resolve creators
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"users":{"edges":[{"node":{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}},{"node":{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":2}}}}`)),
					},
					// Health check
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
					},
					// ("", "", created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"585cf60b-bdc3-45c5-a909-a9dcd0434db7","entity_type":"Email-Addr","observable_value":"abc@test.com","created_at":"2025-06-16T15:45:55.316Z","updated_at":"2025-06-16T15:45:55.316Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", "", updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-01-15T16:17:05.211Z","updated_at":"2025-07-13T11:29:13.100Z","creators":[{"id":"43dfeb13-95d2-4160-9002-d63a6e611aff","name":"reporter"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", admin, created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"585cf60b-bdc3-45c5-a909-a9dcd0434db7","entity_type":"Email-Addr","observable_value":"abc@test.com","created_at":"2025-06-16T15:45:55.316Z","updated_at":"2025-06-16T15:45:55.316Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", admin, updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-02-15T16:17:05.211Z","updated_at":"2025-04-18T13:21:11.816Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", analyst, created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"a087b38d-3f6c-441b-84ba-60bcfe304bde","entity_type":"Hostname","observable_value":"test.com","created_at":"2025-01-16T15:45:55.316Z","updated_at":"2025-01-16T15:45:55.316Z","creators":[{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", analyst, updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"77709e41-f8b0-43e6-bf6e-c33cce9341ec","entity_type":"Hostname","observable_value":"abc.xyz","created_at":"2025-01-06T15:46:55.316Z","updated_at":"2025-05-01T00:12:15.816Z","creators":[{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
				},
			},
			entityTypes:  nil,
			creatorNames: []string{"admin", "analyst"},
			expected: `
			# HELP opencti_last_created_timestamp_seconds Timestamp of the last creation in OpenCTI by entity type and creator.
			# TYPE opencti_last_created_timestamp_seconds gauge
			opencti_last_created_timestamp_seconds{creator="",entity_type=""} 1.750088755e+09
			opencti_last_created_timestamp_seconds{creator="admin",entity_type=""} 1.750088755e+09
			opencti_last_created_timestamp_seconds{creator="analyst",entity_type=""} 1.737042355e+09
			# HELP opencti_last_updated_timestamp_seconds Timestamp of the last update in OpenCTI by entity type and creator.
			# TYPE opencti_last_updated_timestamp_seconds gauge
			opencti_last_updated_timestamp_seconds{creator="",entity_type=""} 1.752406153e+09
			opencti_last_updated_timestamp_seconds{creator="admin",entity_type=""} 1.744982471e+09
			opencti_last_updated_timestamp_seconds{creator="analyst",entity_type=""} 1.746058335e+09
			# HELP opencti_up Wether OpenCTI is up.
			# TYPE opencti_up gauge
			opencti_up 1
			`,
		},
		{
			name: "with entity type and creator filtering",
			transport: &MockTransport{
				Responses: []*http.Response{
					// Resolve creators
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"users":{"edges":[{"node":{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}},{"node":{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":2}}}}`)),
					},
					// Health check
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
					},
					// ("", "", created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-02-15T16:17:05.211Z","updated_at":"2025-01-16T15:47:03.324Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", "", updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"77709e41-f8b0-43e6-bf6e-c33cce9341ec","entity_type":"IPv4-Addr","observable_value":"192.168.1.1","created_at":"2025-01-06T15:46:55.316Z","updated_at":"2025-05-02T10:02:05.100Z","creators":[{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", admin, created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-02-15T16:17:05.211Z","updated_at":"2025-01-16T15:47:03.324Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", admin, updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"585cf60b-bdc3-45c5-a909-a9dcd0434db7","entity_type":"Hostname","observable_value":"both.xyz","created_at":"2025-01-16T15:45:55.316Z","updated_at":"2025-04-18T13:21:11.816Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", analyst, created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"a087b38d-3f6c-441b-84ba-60bcfe304bde","entity_type":"Hostname","observable_value":"test.com","created_at":"2025-01-16T15:45:55.316Z","updated_at":"2025-01-16T15:45:55.316Z","creators":[{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", analyst, updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"77709e41-f8b0-43e6-bf6e-c33cce9341ec","entity_type":"Hostname","observable_value":"abc.xyz","created_at":"2025-01-06T15:46:55.316Z","updated_at":"2025-05-01T00:12:15.816Z","creators":[{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Hostname", "", created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-02-15T16:17:05.211Z","updated_at":"2025-01-16T15:47:03.324Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Hostname", "", updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"77709e41-f8b0-43e6-bf6e-c33cce9341ec","entity_type":"Hostname","observable_value":"abc.xyz","created_at":"2025-01-06T15:46:55.316Z","updated_at":"2025-05-01T00:12:15.816Z","creators":[{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Hostname", admin, created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-02-15T16:17:05.211Z","updated_at":"2025-01-16T15:47:03.324Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Hostname", admin, updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"585cf60b-bdc3-45c5-a909-a9dcd0434db7","entity_type":"Hostname","observable_value":"both.xyz","created_at":"2025-01-16T15:45:55.316Z","updated_at":"2025-04-18T13:21:11.816Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Hostname", analyst, created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"a087b38d-3f6c-441b-84ba-60bcfe304bde","entity_type":"Hostname","observable_value":"test.com","created_at":"2025-01-16T15:45:55.316Z","updated_at":"2025-01-16T15:45:55.316Z","creators":[{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Hostname", analyst, updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"77709e41-f8b0-43e6-bf6e-c33cce9341ec","entity_type":"Hostname","observable_value":"abc.xyz","created_at":"2025-01-06T15:46:55.316Z","updated_at":"2025-05-01T00:12:15.816Z","creators":[{"id":"7aad00b5-baad-4d27-935a-b17e364a3b3b","name":"analyst"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
				},
			},
			entityTypes:  []string{"Hostname"},
			creatorNames: []string{"admin", "analyst"},
			expected: `
			# HELP opencti_last_created_timestamp_seconds Timestamp of the last creation in OpenCTI by entity type and creator.
			# TYPE opencti_last_created_timestamp_seconds gauge
			opencti_last_created_timestamp_seconds{creator="",entity_type=""} 1.739636225e+09
			opencti_last_created_timestamp_seconds{creator="admin",entity_type=""} 1.739636225e+09
			opencti_last_created_timestamp_seconds{creator="analyst",entity_type=""} 1.737042355e+09
			opencti_last_created_timestamp_seconds{creator="",entity_type="Hostname"} 1.739636225e+09
			opencti_last_created_timestamp_seconds{creator="admin",entity_type="Hostname"} 1.739636225e+09
			opencti_last_created_timestamp_seconds{creator="analyst",entity_type="Hostname"} 1.737042355e+09
			# HELP opencti_last_updated_timestamp_seconds Timestamp of the last update in OpenCTI by entity type and creator.
			# TYPE opencti_last_updated_timestamp_seconds gauge
			opencti_last_updated_timestamp_seconds{creator="",entity_type=""} 1.746180125e+09
			opencti_last_updated_timestamp_seconds{creator="admin",entity_type=""} 1.744982471e+09
			opencti_last_updated_timestamp_seconds{creator="analyst",entity_type=""} 1.746058335e+09
			opencti_last_updated_timestamp_seconds{creator="",entity_type="Hostname"} 1.746058335e+09
			opencti_last_updated_timestamp_seconds{creator="admin",entity_type="Hostname"} 1.744982471e+09
			opencti_last_updated_timestamp_seconds{creator="analyst",entity_type="Hostname"} 1.746058335e+09
			# HELP opencti_up Wether OpenCTI is up.
			# TYPE opencti_up gauge
			opencti_up 1
			`,
		},
		{
			name: "no creators, entity types only",
			transport: &MockTransport{
				Responses: []*http.Response{
					// Health check
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
					},
					// ("", "", created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"585cf60b-bdc3-45c5-a909-a9dcd0434db7","entity_type":"Email-Addr","observable_value":"abc@test.com","created_at":"2025-06-16T15:45:55.316Z","updated_at":"2025-06-16T15:45:55.316Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("", "", updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"a55a377c-1605-4ee0-9f1c-6cbaa191edf7","entity_type":"Email-Addr","observable_value":"def@test.com","created_at":"2025-02-01T11:05:30.123Z","updated_at":"2025-07-11T12:13:14.416Z","creators":[{"id":"4efc1502-ca9c-4225-b445-cca1b458a1ce","name":"test"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Email-Addr", "", created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"585cf60b-bdc3-45c5-a909-a9dcd0434db7","entity_type":"Email-Addr","observable_value":"abc@test.com","created_at":"2025-06-16T15:45:55.316Z","updated_at":"2025-06-16T15:45:55.316Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Email-Addr", "", updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"a55a377c-1605-4ee0-9f1c-6cbaa191edf7","entity_type":"Email-Addr","observable_value":"def@test.com","created_at":"2025-02-01T11:05:30.123Z","updated_at":"2025-07-11T12:13:14.416Z","creators":[{"id":"4efc1502-ca9c-4225-b445-cca1b458a1ce","name":"test"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Hostname", "", created_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-01-15T16:17:05.211Z","updated_at":"2025-01-16T15:47:03.324Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
					// ("Hostname", "", updated_at)
					{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"stixCyberObservables":{"edges":[{"node":{"id":"40dd1bb7-0474-4b6f-b2ce-81bf6e692a7a","entity_type":"Hostname","observable_value":"test.xyz","created_at":"2025-01-15T16:17:05.211Z","updated_at":"2025-01-16T15:47:03.324Z","creators":[{"id":"88ec0c6a-13ce-5e39-b486-354fe4a7084f","name":"admin"}]}}],"pageInfo":{"endCursor":"","hasNextPage":false,"globalCount":1}}}}`)),
					},
				},
			},
			entityTypes:  []string{"Email-Addr", "Hostname"},
			creatorNames: nil,
			expected: `
			# HELP opencti_last_created_timestamp_seconds Timestamp of the last creation in OpenCTI by entity type and creator.
			# TYPE opencti_last_created_timestamp_seconds gauge
			opencti_last_created_timestamp_seconds{creator="",entity_type=""} 1.750088755e+09
			opencti_last_created_timestamp_seconds{creator="",entity_type="Email-Addr"} 1.750088755e+09
			opencti_last_created_timestamp_seconds{creator="",entity_type="Hostname"} 1.736957825e+09
			# HELP opencti_last_updated_timestamp_seconds Timestamp of the last update in OpenCTI by entity type and creator.
			# TYPE opencti_last_updated_timestamp_seconds gauge
			opencti_last_updated_timestamp_seconds{creator="",entity_type=""} 1.752235994e+09
			opencti_last_updated_timestamp_seconds{creator="",entity_type="Email-Addr"} 1.752235994e+09
			opencti_last_updated_timestamp_seconds{creator="",entity_type="Hostname"} 1.737042423e+09
			# HELP opencti_up Wether OpenCTI is up.
			# TYPE opencti_up gauge
			opencti_up 1
			`,
		},
		{
			name: "opencti not up during scrape",
			transport: &MockTransport{
				Responses: []*http.Response{
					{
						StatusCode: http.StatusBadGateway,
						Header:     make(http.Header),
						Body:       io.NopCloser(nil),
					},
				},
			},
			creatorNames: nil,
			expected: `
			# HELP opencti_up Wether OpenCTI is up.
			# TYPE opencti_up gauge
			opencti_up 0
			`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			openctiClient, err := gocti.NewOpenCTIAPIClient(
				"https://opencti:8080", "testtoken",
				gocti.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))),
				gocti.WithTransport(test.transport),
			)
			if err != nil {
				t.Fatalf("cannot create OpenCTI client: %v", err)
			}

			coll, err := collector.NewOpenCTICollector(
				context.Background(), openctiClient, "",
				test.entityTypes,
				test.creatorNames,
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
			)
			require.NoError(t, err)

			err = testutil.CollectAndCompare(coll, strings.NewReader(test.expected),
				"opencti_up",
				"opencti_last_created_timestamp_seconds",
				"opencti_last_updated_timestamp_seconds",
			)
			require.NoError(t, err)
		})
	}
}
