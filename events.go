package main

import (
	"fmt"
	"net/http"
	"time"
)

type graphEventStream struct {
	reader     graphReader
	subscriber graphSubscriber
}

func newGraphEventStream(reader graphReader, subscriber graphSubscriber) *graphEventStream {
	return &graphEventStream{reader: reader, subscriber: subscriber}
}

func (stream *graphEventStream) serve(
	response http.ResponseWriter,
	request *http.Request,
	keepAliveInterval time.Duration,
) {
	flusher, supported := response.(http.Flusher)
	if !supported {
		http.Error(response, "event streaming unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("X-Accel-Buffering", "no")

	updates, unsubscribe := stream.subscriber.subscribe()
	defer unsubscribe()
	if err := writeServerEvent(response, "ready", stream.reader.currentGraph().Revision); err != nil {
		return
	}
	flusher.Flush()

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case revision := <-updates:
			if err := writeServerEvent(response, "graph", revision); err != nil {
				return
			}
			flusher.Flush()
		case <-keepAlive.C:
			if _, err := fmt.Fprint(response, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeServerEvent(response http.ResponseWriter, event, data string) error {
	if _, err := fmt.Fprintf(response, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return newUnavailableError("write server event", err)
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T18:28:21Z","module_hash":"c8c2fff4cbe18ac23b9fcd34205f670df9b18b8b88e46128b90a6f8f8e38caec","functions":[{"id":"func/newGraphEventStream","name":"newGraphEventStream","line":14,"end_line":16,"hash":"47c64b33f8baba8e27f3816317ca2d809794d503b35101c287f64a6e699475bf"},{"id":"func/graphEventStream.serve","name":"graphEventStream.serve","line":18,"end_line":58,"hash":"1de355acefc74c8bf39b8ccc87fb534d333ffb2d586a716208cfd066e159d4f6"},{"id":"func/writeServerEvent","name":"writeServerEvent","line":60,"end_line":65,"hash":"df10c28a5b0173429376b5711f7e6728c170c001be27a968fc9eaeb18177aebb"}]}
// mutate4go-manifest-end
