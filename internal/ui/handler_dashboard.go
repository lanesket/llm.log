package ui

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check cache: if MaxID hasn't changed, return cached response
	cacheKey := from.String() + to.String()

	maxID, err := s.store.MaxID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check data freshness")
		return
	}

	s.cacheMu.RLock()
	if maxID == s.cacheMaxID && maxID > 0 && s.cachedKey == cacheKey {
		body := s.cachedBody
		s.cacheMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
		return
	}
	s.cacheMu.RUnlock()

	data, err := s.store.DashboardStats(from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load dashboard stats")
		return
	}

	body, err := json.Marshal(data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}

	// Update cache (single entry, overwrite on key change or data change)
	s.cacheMu.Lock()
	s.cacheMaxID = maxID
	s.cachedKey = cacheKey
	s.cachedBody = body
	s.cacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}
