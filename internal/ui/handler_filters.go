package ui

import "net/http"

func (s *Server) handleFilters(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, err := s.store.Filters(from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load filters")
		return
	}

	writeJSON(w, http.StatusOK, data)
}
