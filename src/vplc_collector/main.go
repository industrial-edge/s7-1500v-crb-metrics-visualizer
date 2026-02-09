/*
 Copyright (c) Siemens 2025
This file is subject to the terms and conditions of the MIT License.
See LICENSE file in the top-level directory.
*/

// main.go
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type VplcAccess struct {
	Name     string `json:"name"`
	LoginUrl string `json:"loginUrl"`
	ApiUrl   string `json:"apiUrl"`
	UserName string `json:"user"`
	Password string `json:"password"`
}

type VplcAccessList struct {
	Instances []VplcAccess `json:"vplcs"`
}

type Histogram struct {
	Timestamp                       int64              `json:"timestampNs"`
	CycleExtensionDurationHistogram map[string]float64 `json:"cycleDelayHistogram"`
	WriteDurationHistogram          map[string]float64 `json:"writeDurationHistogram"`
}

// Metric Descriptor Section (edit here if API changes)
type metricType int

const (
	counter metricType = iota
	gauge
)

type metricDesc struct {
	JSONName string
	PromName string
	Help     string
	Type     metricType
}

var crbMetricDescs = []metricDesc{
	{"writeSuccesses", "crb_successful_cycle_count", "Total number of successful cycles", counter},
	{"writeAttempts", "crb_total_cycle_count", "Total number of cycles", counter},
	{"cycleDelay/totalNs", "crb_sum_cycle_extension_duration", "Sum of cycle extension durations", counter},
	{"cycleDelay/minNs", "crb_min_cycle_extension_duration", "Minimum cycle extension duration", gauge},
	{"cycleDelay/maxNs", "crb_max_cycle_extension_duration", "Maximum cycle extension duration", gauge},
	{"writeDuration/minNs", "crb_min_write_duration", "Minimum write duration", gauge},
	{"writeDuration/maxNs", "crb_max_write_duration", "Maximum write duration", gauge},
	{"writeDuration/totalNs", "crb_sum_write_duration", "Sum of write durations", counter},
}

// Metric Registration and Storage
var crbMetrics = make(map[string]*prometheus.CounterVec)
var crbGauges = make(map[string]*prometheus.GaugeVec)

func initCRBMetrics() {
	for _, desc := range crbMetricDescs {
		switch desc.Type {
		case counter:
			crbMetrics[desc.PromName] = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: desc.PromName,
					Help: desc.Help,
				},
				[]string{"vplc_instance"},
			)
			prometheus.MustRegister(crbMetrics[desc.PromName])
		case gauge:
			crbGauges[desc.PromName] = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: desc.PromName,
					Help: desc.Help,
				},
				[]string{"vplc_instance"},
			)
			prometheus.MustRegister(crbGauges[desc.PromName])
		}
	}
}

// Histogram Metrics
var (
	writeDurationBuckets = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "write_duration_bucket",
			Help: "Write duration histogram buckets (cumulative)",
		},
		[]string{"vplc_instance", "le"},
	)

	cycleExtensionBuckets = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cycle_extension_bucket",
			Help: "Cycle extension histogram buckets (cumulative)",
		},
		[]string{"vplc_instance", "le"},
	)
)

func init() {
	initCRBMetrics()
	prometheus.MustRegister(cycleExtensionBuckets)
	prometheus.MustRegister(writeDurationBuckets)
}

// Helper: parse upper bound from bucket string like "0.0-1.0ms" or "20.0+ms"
func parseBucketUpperBoundMs(bucket string) float64 {
	// Matches "0.0-1.0ms" or "20.0+ms"
	re := regexp.MustCompile(`(\d+(\.\d+)?)(\+)?ms$`)
	matches := re.FindStringSubmatch(bucket)
	if len(matches) >= 2 {
		val, _ := strconv.ParseFloat(matches[1], 64)
		if matches[3] == "+" {
			return 1e9 // treat "+" as a very large bucket
		}
		return val
	}
	return 0
}

// Helper: get value from nested map using slash-separated path
func getNestedValue(data map[string]interface{}, path string) (float64, bool) {
	parts := strings.Split(path, "/")
	var current interface{} = data
	for i, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return 0, false
		}
		current, ok = m[part]
		if !ok {
			return 0, false
		}
		// If last part, try to convert to float64
		if i == len(parts)-1 {
			switch v := current.(type) {
			case float64:
				return v, true
			case int:
				return float64(v), true
			case int64:
				return float64(v), true
			case json.Number:
				f, err := v.Float64()
				if err == nil {
					return f, true
				}
			}
			return 0, false
		}
	}
	return 0, false
}

// Main Metric Update Function
var (
	previousCounterValues = make(map[string]map[string]float64) // map[promName][vplc_instance]value
	prevWriteBuckets      = make(map[string]float64)            // key: vplc_instance|le
	prevCycleBuckets      = make(map[string]float64)            // key: vplc_instance|le
	metricsMu             sync.Mutex
)

func updateCRBMetrics(data map[string]interface{}, vplc_instance string) {
	// v2 api puts all relevant data under performanceMetrics
	performanceData, ok := data["performanceMetrics"].(map[string]interface{})
	if !ok {
		log.Printf("performanceMetrics not found for vplc %s", vplc_instance)
		return
	}
	metricsMu.Lock()
	defer metricsMu.Unlock()
	for _, desc := range crbMetricDescs {
		if v, ok := getNestedValue(performanceData, desc.JSONName); ok {
			switch desc.Type {
			case counter:
				if _, exists := previousCounterValues[desc.PromName]; !exists {
					previousCounterValues[desc.PromName] = make(map[string]float64)
				}
				prevValue := previousCounterValues[desc.PromName][vplc_instance]
				delta := v - prevValue
				if delta > 0 {
					crbMetrics[desc.PromName].WithLabelValues(vplc_instance).Add(delta)
				}
				previousCounterValues[desc.PromName][vplc_instance] = v
			case gauge:
				crbGauges[desc.PromName].WithLabelValues(vplc_instance).Set(v)
			}
		}
	}
}

func updateHistograms(hist Histogram, vplc_instance string) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	// Sort WriteDurationHistogram by upper bound
	var sortedWriteKeys []string
	for k := range hist.WriteDurationHistogram {
		sortedWriteKeys = append(sortedWriteKeys, k)
	}
	sort.Slice(sortedWriteKeys, func(i, j int) bool {
		return parseBucketUpperBoundMs(sortedWriteKeys[i]) < parseBucketUpperBoundMs(sortedWriteKeys[j])
	})
	var cumulative float64
	for _, k := range sortedWriteKeys {
		cumulative += float64(hist.WriteDurationHistogram[k])
		le := strconv.FormatFloat(parseBucketUpperBoundMs(k), 'f', -1, 64)
		curr := writeDurationBuckets.WithLabelValues(vplc_instance, le)
		delta := cumulative - getPreviousBucketValue("write", vplc_instance+"|"+le)
		if delta > 0 {
			curr.Add(delta)
		}
		setPreviousBucketValue("write", vplc_instance+"|"+le, cumulative)
	}

	// Sort CycleExtensionDurationHistogram by upper bound
	var sortedCycleKeys []string
	for k := range hist.CycleExtensionDurationHistogram {
		sortedCycleKeys = append(sortedCycleKeys, k)
	}
	sort.Slice(sortedCycleKeys, func(i, j int) bool {
		return parseBucketUpperBoundMs(sortedCycleKeys[i]) < parseBucketUpperBoundMs(sortedCycleKeys[j])
	})
	cumulative = 0
	for _, k := range sortedCycleKeys {
		cumulative += float64(hist.CycleExtensionDurationHistogram[k])
		le := strconv.FormatFloat(parseBucketUpperBoundMs(k), 'f', -1, 64)
		curr := cycleExtensionBuckets.WithLabelValues(vplc_instance, le)
		delta := cumulative - getPreviousBucketValue("cycle", vplc_instance+"|"+le)
		if delta > 0 {
			curr.Add(delta)
		}
		setPreviousBucketValue("cycle", vplc_instance+"|"+le, cumulative)
	}
}

func getPreviousBucketValue(kind, key string) float64 {
	switch kind {
	case "write":
		return prevWriteBuckets[key]
	case "cycle":
		return prevCycleBuckets[key]
	}
	return 0
}

func setPreviousBucketValue(kind, key string, val float64) {
	switch kind {
	case "write":
		prevWriteBuckets[key] = val
	case "cycle":
		prevCycleBuckets[key] = val
	}
}

// Update readAccessFile to return a list of VplcAccess
func readAccessFile() []VplcAccess {
	filePath := os.Getenv("VPLC_ACCESS_FILE")
	if filePath == "" {
		log.Fatalf("Environment variable VPLC_ACCESS_FILE is not set or empty")
	}
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening access file: %v", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("Error reading access file: %v", err)
	}
	var accessList VplcAccessList
	if err := json.Unmarshal(data, &accessList); err != nil {
		log.Fatalf("Error parsing access file: %v", err)
	}
	return accessList.Instances
}

func authenticate(client *http.Client, loginUrl, username, password string) (string, error) {
	loginData := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
	req, err := http.NewRequest("POST", loginUrl, strings.NewReader(loginData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("authentication failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	token, ok := result["accessToken"].(string)
	if !ok {
		return "", fmt.Errorf("no access token in response")
	}

	log.Printf("Authentication successful: %s", loginUrl)
	return token, nil
}

func sendApiRequest(client *http.Client, url, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Cookie", "authToken="+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return body, nil
}

func main() {
	vplcList := readAccessFile()
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}

	for _, vplc := range vplcList {
		go func(vplc VplcAccess) {
			vplc_instance := vplc.Name
			var token string
			var authenticated bool
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			var running int32 // 0 = not running, 1 = running

			for range ticker.C {
				if !atomic.CompareAndSwapInt32(&running, 0, 1) {
					// Previous run still in progress, skip this tick
					log.Printf("Previous run still in progress for vplc %s, skipping this interval", vplc_instance)
					continue
				}

				go func() {
					defer atomic.StoreInt32(&running, 0)
					defer func() {
						if r := recover(); r != nil {
							log.Printf("Recovered from panic for vplc %s: %v", vplc_instance, r)
							authenticated = false
							token = ""
						}
					}()

					// Check if we need to authenticate
					if !authenticated || token == "" {
						var err error
						token, err = authenticate(client, vplc.LoginUrl, vplc.UserName, vplc.Password)
						if err != nil {
							log.Printf("Authentication failed for vplc %s: %v", vplc_instance, err)
							authenticated = false
							return // Skip this cycle
						}
						authenticated = true
					}

					// Try to get histogram data
					body, err := sendApiRequest(client, vplc.ApiUrl+"/retain/cyclic-backup/histogram", token)
					if err != nil {
						log.Printf("Failed to get histogram data for vplc %s: %v", vplc_instance, err)
						authenticated = false
						token = ""
						return
					}

					var hist Histogram
					if err := json.Unmarshal(body, &hist); err != nil {
						log.Printf("Error parsing histogram data for vplc %s: %v", vplc_instance, err)
					} else {
						updateHistograms(hist, vplc_instance)
					}

					// Try to get CRB stats
					bodyStats, err := sendApiRequest(client, vplc.ApiUrl+"/retain/cyclic-backup", token)
					if err != nil {
						log.Printf("Failed to get CRB stats for vplc %s: %v", vplc_instance, err)
						authenticated = false // Mark as unauthenticated to retry auth next cycle
						token = ""
						return
					}

					var crb map[string]interface{}
					if err := json.Unmarshal(bodyStats, &crb); err != nil {
						log.Printf("Error parsing CRB metrics for vplc %s: %v", vplc_instance, err)
					} else {
						updateCRBMetrics(crb, vplc_instance)
					}
				}()
			}
		}(vplc)
	}

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Serving metrics on :2112/metrics")
	log.Fatal(http.ListenAndServe(":2112", nil))
}
