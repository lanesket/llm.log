package ui

import "net/http"

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	granularity := r.URL.Query().Get("granularity")
	// Validate granularity
	switch granularity {
	case "minute", "hour", "day", "week", "":
		// valid
	default:
		writeError(w, http.StatusBadRequest, "granularity must be one of: minute, hour, day, week")
		return
	}

	data, err := s.store.Analytics(from, to, granularity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}

	writeJSON(w, http.StatusOK, data)
}
