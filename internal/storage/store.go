package storage

import "time"

// Record represents a single intercepted LLM API call.
type Record struct {
	ID               int64
	Timestamp        time.Time
	Provider         string
	Model            string
	Endpoint         string
	Source           string // "cc:sub", "cc:key", "" (unknown)
	InputTokens      int    // total input (includes cache read + write)
	OutputTokens     int
	CacheReadTokens  int // tokens read from cache
	CacheWriteTokens int // tokens written to cache (Anthropic only)
	TotalCost        *float64
	DurationMs       int
	Streaming        bool
	StatusCode       int
	RequestBody      []byte // populated only by Get()
	ResponseBody     []byte // populated only by Get()
}

// StatRow is a single row from an aggregated stats query.
type StatRow struct {
	Key              string
	Provider         string // provider name (useful when GroupBy is "model" or "day")
	Requests         int
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	TotalCost        float64
	AvgDurationMs    int
	Errors           int // non-200 status codes
}

// StatsFilter controls what stats are returned.
type StatsFilter struct {
	From     time.Time
	To       time.Time
	GroupBy  string // "provider", "model", "day"
	Provider string
	Model    string
	Source   string
}

// ListFilter controls paginated request listing.
type ListFilter struct {
	From       time.Time
	To         time.Time
	Provider   string
	Model      string
	Source     string
	StatusCode int   // 0 means no filter
	Streaming  *bool // nil means no filter
	MinCost    *float64
	MaxCost    *float64
	MinTokens  *int
	MaxTokens  *int
	SortBy     string  // "timestamp", "cost", "input_tokens", "output_tokens", "duration_ms"
	SortDir    string  // "asc", "desc"
	Cursor     string  // opaque base64 cursor
	Limit      int     // default 50, max 200
	SearchIDs  []int64 // pre-filtered IDs from body search
}

// ListResult holds paginated results.
type ListResult struct {
	Items      []Record
	Total      *int   // nil when body search active (SearchIDs set)
	NextCursor string // empty when no more results
}

// PruneStats holds information about bodies eligible for pruning.
type PruneStats struct {
	Count int64
	Bytes int64
}

// Totals holds aggregate metrics for a time range.
type Totals struct {
	Requests            int     `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheWriteTokens    int64   `json:"cache_write_tokens"`
	TotalCost           float64 `json:"total_cost"`
	UnknownCostRequests int     `json:"unknown_cost_requests"`
	Errors              int     `json:"errors"`
	AvgDurationMs       int     `json:"avg_duration_ms"`
}

// BreakdownRow is a single row in a breakdown (by provider, model, or source).
type BreakdownRow struct {
	Name      string  `json:"name"`
	Provider  string  `json:"provider,omitempty"`
	Requests  int     `json:"requests"`
	TotalCost float64 `json:"total_cost"`
	Tokens    int64   `json:"tokens"`
}

// ChartPoint is a single time-bucketed data point.
type ChartPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Requests  int       `json:"requests"`
	Cost      float64   `json:"cost"`
	Tokens    int64     `json:"tokens"`
}

// DailyActivity is one day's aggregated activity for the contribution heatmap.
type DailyActivity struct {
	Date     string  `json:"date"` // "2006-01-02"
	Requests int     `json:"requests"`
	Cost     float64 `json:"cost"`
}

// DashboardData holds all data needed to render the dashboard.
type DashboardData struct {
	Totals     Totals          `json:"totals"`
	PrevTotals *Totals         `json:"prev_totals"`
	ByProvider []BreakdownRow  `json:"by_provider"`
	ByModel    []BreakdownRow  `json:"by_model"`
	BySource   []BreakdownRow  `json:"by_source"`
	Chart      []ChartPoint    `json:"chart"`
	Activity   []DailyActivity `json:"activity"`
}

// AnalyticsPoint is a time-bucketed aggregation point.
type AnalyticsPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	Requests         int       `json:"requests"`
	Cost             float64   `json:"cost"`
	InputTokens      int64     `json:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens"`
}

// ProviderTimePoint is a time-bucketed aggregation by provider.
type ProviderTimePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Provider  string    `json:"provider"`
	Requests  int       `json:"requests"`
	Cost      float64   `json:"cost"`
}

// ModelStat holds aggregated stats for a model.
type ModelStat struct {
	Model         string  `json:"model"`
	Provider      string  `json:"provider"`
	Requests      int     `json:"requests"`
	Cost          float64 `json:"cost"`
	AvgDurationMs int     `json:"avg_duration_ms"`
}

// ExpensiveRequest identifies a high-cost request.
type ExpensiveRequest struct {
	ID          int64     `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Model       string    `json:"model"`
	Cost        float64   `json:"cost"`
	TotalTokens int64     `json:"total_tokens"`
}

// CostDistribution holds cost percentiles.
type CostDistribution struct {
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Max float64 `json:"max"`
}

// HeatmapEntry holds request count and cost for a day/hour cell.
type HeatmapEntry struct {
	DayOfWeek int     `json:"day_of_week"`
	Hour      int     `json:"hour"`
	Requests  int     `json:"requests"`
	Cost      float64 `json:"cost"`
}

// CumulativePoint is a time-bucketed running cost total.
type CumulativePoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Cumulative float64   `json:"cumulative"`
}

// CacheHitPoint is a time-bucketed cache hit rate.
type CacheHitPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Rate      float64   `json:"rate"`
}

// AvgTokensRow holds average token counts per model.
type AvgTokensRow struct {
	Model     string  `json:"model"`
	AvgInput  float64 `json:"avg_input"`
	AvgOutput float64 `json:"avg_output"`
}

// AnalyticsData holds all analytics data for the analytics page.
type AnalyticsData struct {
	OverTime           []AnalyticsPoint    `json:"over_time"`
	ByProviderOverTime []ProviderTimePoint `json:"by_provider_over_time"`
	TopModels          []ModelStat         `json:"top_models"`
	TopExpensive       []ExpensiveRequest  `json:"top_expensive"`
	CostDistribution   CostDistribution    `json:"cost_distribution"`
	CumulativeCost     []CumulativePoint   `json:"cumulative_cost"`
	CacheHitRate       []CacheHitPoint     `json:"cache_hit_rate"`
	AvgTokensPerReq    []AvgTokensRow      `json:"avg_tokens_per_request"`
	Heatmap            []HeatmapEntry      `json:"heatmap"`
}

// FilterOptions holds distinct values for filter dropdowns.
type FilterOptions struct {
	Providers []string `json:"providers"`
	Models    []string `json:"models"`
	Sources   []string `json:"sources"`
}

// Store is the storage interface.
type Store interface {
	Save(rec *Record) error
	SaveBatch(recs []*Record) error
	Stats(f StatsFilter) ([]StatRow, error)
	Recent(n int, from, to time.Time, provider, source string) ([]Record, error)
	List(f ListFilter) (*ListResult, error)
	Get(id int64) (*Record, error)
	Sources(from, to time.Time) ([]string, error)
	DashboardStats(from, to time.Time) (*DashboardData, error)
	Analytics(from, to time.Time, granularity string) (*AnalyticsData, error)
	SearchByBody(query string, from, to time.Time, maxScan, limit int) ([]int64, error)
	Filters(from, to time.Time) (*FilterOptions, error)
	MaxID() (int64, error)
	PrunePreview(before time.Time) (PruneStats, error)
	PruneBodies(before time.Time) (int64, error)
	Vacuum() error
	Close() error
}
