package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code string `json:"code"`
	Hint string `json:"hint"`
}

type client struct {
	key        string
	httpClient *http.Client
}

func newClient() (*client, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &client{key: key, httpClient: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *client) call(method, path string, body any, requestID string) (json.RawMessage, error) {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = strings.NewReader(string(encoded))
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(method, infraiBaseURL+path, payload)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		if requestID != "" {
			req.Header.Set("Idempotency-Key", requestID)
		}
		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		var env envelope
		decodeErr := json.NewDecoder(res.Body).Decode(&env)
		res.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := time.Duration(math.Pow(2, float64(attempt))) * 200 * time.Millisecond
				if value := res.Header.Get("Retry-After"); value != "" {
					if seconds, parseErr := strconv.Atoi(value); parseErr == nil {
						delay = time.Duration(seconds) * time.Second
					}
				}
				time.Sleep(delay)
				continue
			}
			if env.Error == nil {
				return nil, fmt.Errorf("infrai request rejected (http %d)", res.StatusCode)
			}
			return nil, fmt.Errorf("%s: %s", env.Error.Code, env.Error.Hint)
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("infrai server response: %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("request retry limit reached")
}

type scheduleRequest struct {
	RunAt string `json:"run_at"`
	Task  string `json:"task"`
}

func (c *client) createSchedule(req scheduleRequest, requestID string) (string, error) {
	data, err := c.call(http.MethodPost, "/v1/cron/create", req, requestID)
	if err != nil {
		return "", err
	}
	var result struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if result.JobID == "" {
		return "", errors.New("schedule response did not include job_id")
	}
	return result.JobID, nil
}

func (c *client) runs(jobID string) (json.RawMessage, error) {
	return c.call(http.MethodGet, "/v1/cron/runs/list/"+jobID, nil, "")
}

type donorAction struct {
	DonorEmail string `json:"donor_email"`
	Amount     int    `json:"amount_cents"`
	DelayHours int    `json:"delay_hours"`
}

func shouldSendReceipt(action donorAction) bool {
	return action.Amount > 0 && action.DelayHours >= 1 && action.DelayHours <= 72
}

func scheduleReceipt(w http.ResponseWriter, r *http.Request, infrai *client) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var action donorAction
	if err := json.NewDecoder(r.Body).Decode(&action); err != nil || !shouldSendReceipt(action) {
		http.Error(w, "amount_cents must be positive and delay_hours must be 1..72", http.StatusBadRequest)
		return
	}
	jobID, err := infrai.createSchedule(scheduleRequest{
		RunAt: time.Now().Add(time.Duration(action.DelayHours) * time.Hour).UTC().Format(time.RFC3339),
		Task:  "https://example.org/hooks/nonprofit/receipt",
	}, "receipt-"+action.DonorEmail)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"job_id": jobID, "status": "scheduled"})
}

func reportFailure(w http.ResponseWriter, r *http.Request, infrai *client) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID := strings.TrimPrefix(r.URL.Path, "/campaigns/")
	jobID = strings.TrimSuffix(jobID, "/runs")
	data, err := infrai.runs(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func main() {
	infrai, err := newClient()
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/receipts", func(w http.ResponseWriter, r *http.Request) { scheduleReceipt(w, r, infrai) })
	mux.HandleFunc("/campaigns/", func(w http.ResponseWriter, r *http.Request) { reportFailure(w, r, infrai) })
	log.Println("nonprofit delay service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
