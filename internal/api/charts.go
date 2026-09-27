package api

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/niski84/ble-watch/internal/store"
)

type chartPoint struct {
	Ts   int64   `json:"ts"`
	RSSI float64 `json:"rssi"`
}

type histBin struct {
	Lo    int `json:"lo"`
	Hi    int `json:"hi"`
	Count int `json:"count"`
}

type presenceSpan struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type chartData struct {
	Series    []chartPoint   `json:"series"`
	Histogram []histBin      `json:"histogram"`
	Present   []presenceSpan `json:"present"`
	Mean      float64        `json:"mean"`
	Std       float64        `json:"std"`
}

// handleChart returns downsampled series + histogram + presence for a device.
// Query: ?hours=N (default 24, capped 168).
func (s *Server) handleChart(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	hours := 24
	if v := r.URL.Query().Get("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	if hours > 168 {
		hours = 168
	}
	until := time.Now().Unix()
	since := until - int64(hours*3600)

	raw, err := s.store.Series(mac, since, until)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load series")
		return
	}

	window := until - since
	// Target ~240 points for the line chart.
	bucket := window / 240
	if bucket < 1 {
		bucket = 1
	}

	series := downsample(raw, bucket)
	hist := histogram(raw)
	present := presence(raw, int64(s.cfg.GoneAfter.Seconds()))

	var mean, std float64
	if dev, err := s.store.GetDevice(mac); err == nil {
		mean, std = dev.BaselineMean, dev.BaselineStd
	}

	respondJSON(w, http.StatusOK, chartData{Series: series, Histogram: hist, Present: present, Mean: mean, Std: std})
}

// downsample averages RSSI into fixed-duration buckets.
func downsample(raw []store.Observation, bucket int64) []chartPoint {
	if len(raw) == 0 {
		return []chartPoint{}
	}
	type acc struct {
		sum float64
		n   int
		ts  int64
	}
	buckets := make(map[int64]*acc)
	for _, o := range raw {
		key := o.Ts / bucket * bucket
		a, ok := buckets[key]
		if !ok {
			a = &acc{ts: key}
			buckets[key] = a
		}
		a.sum += o.RSSI
		a.n++
	}
	keys := make([]int64, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]chartPoint, 0, len(keys))
	for _, k := range keys {
		a := buckets[k]
		out = append(out, chartPoint{Ts: a.ts, RSSI: a.sum / float64(a.n)})
	}
	return out
}

// histogram bins RSSI into 20 buckets over [-100, -20] dBm.
func histogram(raw []store.Observation) []histBin {
	const (
		lo   = -100.0
		hi   = -20.0
		bins = 20
	)
	width := (hi - lo) / bins
	out := make([]histBin, bins)
	for i := range out {
		out[i] = histBin{Lo: int(lo + float64(i)*width), Hi: int(lo + float64(i+1)*width)}
	}
	for _, o := range raw {
		r := o.RSSI
		if r < lo {
			r = lo
		}
		if r > hi {
			r = hi - 0.01
		}
		idx := int((r - lo) / width)
		if idx < 0 {
			idx = 0
		}
		if idx >= bins {
			idx = bins - 1
		}
		out[idx].Count++
	}
	return out
}

// presence returns contiguous online spans, splitting on gaps > goneAfter.
func presence(raw []store.Observation, goneAfter int64) []presenceSpan {
	if len(raw) == 0 {
		return []presenceSpan{}
	}
	ts := make([]int64, 0, len(raw))
	for _, o := range raw {
		ts = append(ts, o.Ts)
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })

	out := []presenceSpan{{Start: ts[0], End: ts[0]}}
	for _, t := range ts[1:] {
		last := &out[len(out)-1]
		if t-last.End <= goneAfter {
			last.End = t
		} else {
			out = append(out, presenceSpan{Start: t, End: t})
		}
	}
	return out
}

// keep math import used (guard against future edits dropping it)
var _ = math.MaxInt
