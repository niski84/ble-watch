package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/niski84/ble-watch/internal/store"
	"github.com/niski84/ble-watch/web"
)

// dashboardData builds the dashboard view model from the store.
func (s *Server) dashboardData() web.DashboardData {
	devs, _ := s.store.ListDevices()
	events, _ := s.store.ListEvents(40)
	now := time.Now().Unix()

	data := web.DashboardData{
		Devices: make([]web.DeviceCard, 0, len(devs)),
		Events:  make([]web.EventCard, 0, len(events)),
	}
	for _, d := range devs {
		card := deviceToCard(d, now, s.cfg.GoneAfter)
		data.Devices = append(data.Devices, card)
		if card.Present {
			data.PresentCount++
		}
		if card.Recognized {
			data.RecognizedCount++
		}
	}
	data.TotalDevices = len(devs)
	for _, e := range events {
		data.Events = append(data.Events, eventToCard(e))
	}
	return data
}

// handleIndex serves the live dashboard.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	web.Dashboard(s.dashboardData()).Render(r.Context(), w)
}

// handlePartialsDevices returns the live device grid + stats fragment (SSE refresh).
func (s *Server) handlePartialsDevices(w http.ResponseWriter, r *http.Request) {
	web.DeviceGrid(s.dashboardData()).Render(r.Context(), w)
}

// handlePartialsEvents returns the live event feed fragment (SSE refresh).
func (s *Server) handlePartialsEvents(w http.ResponseWriter, r *http.Request) {
	events, _ := s.store.ListEvents(40)
	cards := make([]web.EventCard, 0, len(events))
	for _, e := range events {
		cards = append(cards, eventToCard(e))
	}
	web.EventFeed(cards).Render(r.Context(), w)
}

// handleDevicePage serves the per-device detail page.
func (s *Server) handleDevicePage(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	dev, err := s.store.GetDevice(mac)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load device")
		return
	}
	events, _ := s.store.ListEvents(500)
	data := web.DevicePageData{
		Device: deviceToCard(*dev, time.Now().Unix(), s.cfg.GoneAfter),
		Events: make([]web.EventCard, 0),
	}
	for _, e := range events {
		if e.Mac == mac {
			data.Events = append(data.Events, eventToCard(e))
		}
	}
	web.DevicePage(data).Render(r.Context(), w)
}

// handleDevicesPage serves the recognized-device management page.
func (s *Server) handleDevicesPage(w http.ResponseWriter, r *http.Request) {
	devs, _ := s.store.ListDevices()
	now := time.Now().Unix()
	data := web.DevicesPageData{Devices: make([]web.DeviceCard, 0, len(devs))}
	for _, d := range devs {
		card := deviceToCard(d, now, s.cfg.GoneAfter)
		data.Devices = append(data.Devices, card)
		if card.Recognized {
			data.RecognizedCount++
		} else {
			data.UnknownCount++
		}
	}
	web.DevicesPage(data).Render(r.Context(), w)
}

// handleEventsPage serves the full event log.
func (s *Server) handleEventsPage(w http.ResponseWriter, r *http.Request) {
	events, _ := s.store.ListEvents(200)
	data := web.EventsPageData{Events: make([]web.EventCard, 0, len(events))}
	for _, e := range events {
		data.Events = append(data.Events, eventToCard(e))
	}
	web.EventsPage(data).Render(r.Context(), w)
}

// deviceToCard converts a store device to a view card.
func deviceToCard(d store.Device, now int64, goneAfter time.Duration) web.DeviceCard {
	return web.DeviceCard{
		Mac:          d.Mac,
		Source:       d.Source,
		Name:         d.Name,
		Group:        d.Group,
		AddressType:  d.AddressType,
		Manufacturer: d.Manufacturer,
		LastSeenAt:   time.Unix(d.LastSeenAt, 0).Format("15:04:05"),
		LastSeenUnix: d.LastSeenAt,
		FirstSeenAt:  time.Unix(d.FirstSeenAt, 0).Format("2006-01-02 15:04"),
		SeenCount:    d.SeenCount,
		LastRSSI:     d.LastRSSI,
		Recognized:   d.Recognized,
		Present:      now-d.LastSeenAt < int64(goneAfter.Seconds()),
		BaselineMean: d.BaselineMean,
		BaselineStd:  d.BaselineStd,
		BaselineN:    d.BaselineN,
	}
}
