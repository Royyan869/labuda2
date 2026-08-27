// Package application provides SLA metrics aggregation for admin dashboard.
package application

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	disputeApp "github.com/labuda/backend/internal/governance/dispute/application"
	"github.com/labuda/backend/internal/governance/dispute/entity"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	supportRepo "github.com/labuda/backend/internal/governance/support/repository"
)

// SLAService aggregates SLA metrics from disputes and support tickets.
type SLAService struct {
	transactor        Transactor
	disputeRepo       disputeRepo.DisputeRepository
	supportRepo       supportRepo.Repository
	disputeSLAService *disputeApp.DisputeSLAService
}


// NewSLAService creates a new SLA service.
func NewSLAService(
	transactor Transactor,
	disputeRepo disputeRepo.DisputeRepository,
	supportRepo supportRepo.Repository,
) *SLAService {
	return &SLAService{
		transactor:        transactor,
		disputeRepo:       disputeRepo,
		supportRepo:       supportRepo,
		disputeSLAService: disputeApp.NewDisputeSLAService(),
	}
}

// SLAMetrics contains aggregated SLA metrics with trends and health.
type SLAMetrics struct {
	Support         *SLAMetricBreakdown        `json:"support"`
	Dispute         *SLAMetricBreakdown        `json:"dispute"`
	AdminPerformance []AdminPerformanceMetrics `json:"admin_performance"`
	SystemHealth    SystemHealthStatus         `json:"system_health"`
	Trends          *TrendComparison           `json:"trends,omitempty"`
	GeneratedAt     time.Time                  `json:"generated_at"`
}

// SLAMetricBreakdown contains enhanced SLA metrics for a domain.
type SLAMetricBreakdown struct {
	// Timing metrics
	AvgFirstResponseTime  *time.Duration `json:"avg_first_response_time"`
	P95FirstResponseTime  *time.Duration `json:"p95_first_response_time"`
	AvgResolutionTime     *time.Duration `json:"avg_resolution_time"`
	P95ResolutionTime     *time.Duration `json:"p95_resolution_time"`

	// Status metrics
	OverdueRate   float64 `json:"overdue_rate"`
	OverdueCount  int64   `json:"overdue_count"`
	TotalCount    int64   `json:"total_count"`

	// Context metrics
	ActiveCount    int64   `json:"active_count"`
	ResolvedCount  int64   `json:"resolved_count"`

	// Health status
	HealthStatus   string  `json:"health_status"` // "good", "warning", "critical"
}

// AdminPerformanceMetrics contains per-admin SLA metrics with health.
type AdminPerformanceMetrics struct {
	AdminID            uuid.UUID      `json:"admin_id"`
	AvgResponseTime    *time.Duration `json:"avg_response_time"`
	P95ResponseTime    *time.Duration `json:"p95_response_time"`
	AvgResolutionTime  *time.Duration `json:"avg_resolution_time"`
	P95ResolutionTime  *time.Duration `json:"p95_resolution_time"`
	OverdueCount       int64          `json:"overdue_count"`
	OverdueRate        float64        `json:"overdue_rate"`
	HandledTickets     int64          `json:"handled_tickets"`
	ActiveWorkload     int64          `json:"active_workload"`
	HealthStatus       string         `json:"health_status"`
}

// SystemHealthStatus represents overall system health.
type SystemHealthStatus struct {
	Status    string  `json:"status"`    // "good", "warning", "critical"
	Score     float64 `json:"score"`     // 0-100
	Issues    []string `json:"issues,omitempty"`
}

// TrendComparison contains metrics for trend analysis.
type TrendComparison struct {
	Last24Hours      *SLAMetricBreakdown `json:"last_24_hours"`
	Previous24Hours  *SLAMetricBreakdown `json:"previous_24_hours"`
	ResponseTimeChange   float64 `json:"response_time_change"`   // percentage
	ResolutionTimeChange float64 `json:"resolution_time_change"` // percentage
	OverdueRateChange    float64 `json:"overdue_rate_change"`    // percentage
}

// GetSLAMetrics returns aggregated SLA metrics with enhanced analytics.
func (s *SLAService) GetSLAMetrics(ctx context.Context) (*SLAMetrics, error) {
	var supportMetrics, disputeMetrics *SLAMetricBreakdown
	var adminPerf []AdminPerformanceMetrics
	var trends *TrendComparison

	// Query metrics within a transaction for consistency
	err := s.transactor.WithTx(ctx, func(tx db.Tx) error {
		var err error

		// Get current period metrics
		supportMetrics, err = s.computeSupportMetrics(ctx, tx, time.Now().Add(-24*time.Hour), nil)
		if err != nil {
			return err
		}

		disputeMetrics, err = s.computeDisputeMetrics(ctx, tx, time.Now().Add(-24*time.Hour), nil)
		if err != nil {
			return err
		}

		// Get admin performance
		adminPerf, err = s.computeAdminPerformance(ctx, tx)
		if err != nil {
			return err
		}

		// Compute trends
		trends, err = s.computeTrends(ctx, tx)
		if err != nil {
			return err // Non-fatal, can continue
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Compute system health
	systemHealth := s.computeSystemHealth(supportMetrics, disputeMetrics, adminPerf)

	return &SLAMetrics{
		Support:          supportMetrics,
		Dispute:          disputeMetrics,
		AdminPerformance: adminPerf,
		SystemHealth:     systemHealth,
		Trends:           trends,
		GeneratedAt:      time.Now(),
	}, nil
}

// computeSupportMetrics calculates enhanced SLA metrics for support tickets.
func (s *SLAService) computeSupportMetrics(ctx context.Context, tx db.Tx, since time.Time, until *time.Time) (*SLAMetricBreakdown, error) {
	// Get all tickets
	tickets, err := s.supportRepo.ListTickets(ctx, tx, nil, nil, nil, 1000)
	if err != nil {
		return nil, err
	}

	// Filter by time range if specified
	var filteredTickets []*supportEntity.Ticket
	if !since.IsZero() {
		for _, ticket := range tickets {
			if until != nil && ticket.ResolvedAt != nil && ticket.ResolvedAt.After(*until) {
				continue
			}
			if ticket.CreatedAt.After(since) || ticket.ResolvedAt != nil && ticket.ResolvedAt.After(since) {
				filteredTickets = append(filteredTickets, ticket)
			}
		}
	} else {
		filteredTickets = tickets
	}

	var firstResponseTimes []time.Duration
	var resolutionTimes []time.Duration
	var overdueCount int64
	var activeCount int64
	var resolvedCount int64

	for _, ticket := range filteredTickets {
		// Compute SLA metrics
		metrics := ticket.ComputeSLAMetricsSimple()

		// First response metrics
		if metrics.FirstResponseTime != nil {
			firstResponseTimes = append(firstResponseTimes, *metrics.FirstResponseTime)
		}

		// Resolution metrics
		if metrics.ResolutionTime != nil {
			resolutionTimes = append(resolutionTimes, *metrics.ResolutionTime)
		}

		// Status counts
		if ticket.IsResolved() {
			resolvedCount++
		} else if ticket.IsOpen() || ticket.Status == supportEntity.StatusInProgress {
			activeCount++
		}

		// Overdue count
		if metrics.IsOverdue {
			overdueCount++
		}
	}

	metrics := &SLAMetricBreakdown{
		TotalCount:   int64(len(filteredTickets)),
		OverdueCount: overdueCount,
		ActiveCount:  activeCount,
		ResolvedCount: resolvedCount,
	}

	if len(firstResponseTimes) > 0 {
		avg := s.averageDuration(firstResponseTimes)
		metrics.AvgFirstResponseTime = &avg
		p95 := s.percentileDuration(firstResponseTimes, 95)
		metrics.P95FirstResponseTime = &p95
	}

	if len(resolutionTimes) > 0 {
		avg := s.averageDuration(resolutionTimes)
		metrics.AvgResolutionTime = &avg
		p95 := s.percentileDuration(resolutionTimes, 95)
		metrics.P95ResolutionTime = &p95
	}

	if len(filteredTickets) > 0 {
		metrics.OverdueRate = float64(overdueCount) / float64(len(filteredTickets))
	}

	// Compute health status
	metrics.HealthStatus = s.computeMetricHealth(metrics.OverdueRate, metrics.AvgResolutionTime, 1*time.Hour)

	return metrics, nil
}

// computeDisputeMetrics calculates enhanced SLA metrics for disputes.
func (s *SLAService) computeDisputeMetrics(ctx context.Context, tx db.Tx, since time.Time, until *time.Time) (*SLAMetricBreakdown, error) {
	// Get all disputes
	disputes, _, err := s.disputeRepo.ListAll(ctx, tx, disputeRepo.DisputeListFilters{
		Page:     1,
		PageSize: 1000,
	})
	if err != nil {
		return nil, err
	}

	// Filter by time range if specified
	var filteredDisputes []*entity.Dispute
	if !since.IsZero() {
		for _, dispute := range disputes {
			if until != nil && dispute.ResolvedAt != nil && dispute.ResolvedAt.After(*until) {
				continue
			}
			if dispute.OpenedAt.After(since) || dispute.ResolvedAt != nil && dispute.ResolvedAt.After(since) {
				filteredDisputes = append(filteredDisputes, dispute)
			}
		}
	} else {
		filteredDisputes = disputes
	}

	var responseTimes []time.Duration
	var resolutionTimes []time.Duration
	var overdueCount int64
	var activeCount int64
	var resolvedCount int64

	for _, dispute := range filteredDisputes {
		// Compute SLA metrics
		metrics := s.disputeSLAService.ComputeMetrics(dispute)

		// Admin response metrics
		if metrics.AdminResponseTime != nil {
			responseTimes = append(responseTimes, *metrics.AdminResponseTime)
		}

		// Resolution metrics
		if metrics.ResolutionTime != nil {
			resolutionTimes = append(resolutionTimes, *metrics.ResolutionTime)
		}

		// Status counts
		if dispute.IsResolved() {
			resolvedCount++
		} else if dispute.IsUnderReview() {
			activeCount++
		}

		// Overdue count
		if metrics.AdminResponseOverdue || metrics.ResolutionOverdue {
			overdueCount++
		}
	}

	metrics := &SLAMetricBreakdown{
		TotalCount:   int64(len(filteredDisputes)),
		OverdueCount: overdueCount,
		ActiveCount:  activeCount,
		ResolvedCount: resolvedCount,
	}

	if len(responseTimes) > 0 {
		avg := s.averageDuration(responseTimes)
		metrics.AvgFirstResponseTime = &avg
		p95 := s.percentileDuration(responseTimes, 95)
		metrics.P95FirstResponseTime = &p95
	}

	if len(resolutionTimes) > 0 {
		avg := s.averageDuration(resolutionTimes)
		metrics.AvgResolutionTime = &avg
		p95 := s.percentileDuration(resolutionTimes, 95)
		metrics.P95ResolutionTime = &p95
	}

	if len(filteredDisputes) > 0 {
		metrics.OverdueRate = float64(overdueCount) / float64(len(filteredDisputes))
	}

	// Compute health status (disputes have longer SLA - 48 hours)
	metrics.HealthStatus = s.computeMetricHealth(metrics.OverdueRate, metrics.AvgResolutionTime, 48*time.Hour)

	return metrics, nil
}

// computeAdminPerformance calculates enhanced per-admin SLA metrics.
func (s *SLAService) computeAdminPerformance(ctx context.Context, tx db.Tx) ([]AdminPerformanceMetrics, error) {
	// Get all admins
	admins, err := s.supportRepo.ListAdmins(ctx, tx, nil)
	if err != nil {
		return nil, err
	}

	adminMetrics := make([]AdminPerformanceMetrics, 0, len(admins))

	for _, admin := range admins {
		// Get tickets assigned to this admin
		tickets, err := s.supportRepo.ListTickets(ctx, tx, &supportRepo.TicketFilter{
			AssignedAdminID: &admin.ID,
		}, nil, nil, 1000)
		if err != nil {
			continue
		}

		var responseTimes []time.Duration
		var resolutionTimes []time.Duration
		var overdueCount int64
		var activeWorkload int64

		for _, ticket := range tickets {
			metrics := ticket.ComputeSLAMetricsSimple()

			if metrics.FirstResponseTime != nil {
				responseTimes = append(responseTimes, *metrics.FirstResponseTime)
			}

			if metrics.ResolutionTime != nil {
				resolutionTimes = append(resolutionTimes, *metrics.ResolutionTime)
			}

			if metrics.IsOverdue {
				overdueCount++
			}

			// Count active workload
			if ticket.IsOpen() || ticket.Status == supportEntity.StatusInProgress {
				activeWorkload++
			}
		}

		perf := AdminPerformanceMetrics{
			AdminID:        admin.ID,
			HandledTickets: int64(len(tickets)),
			OverdueCount:   overdueCount,
			ActiveWorkload: activeWorkload,
		}

		if len(responseTimes) > 0 {
			avg := s.averageDuration(responseTimes)
			perf.AvgResponseTime = &avg
			p95 := s.percentileDuration(responseTimes, 95)
			perf.P95ResponseTime = &p95
		}

		if len(resolutionTimes) > 0 {
			avg := s.averageDuration(resolutionTimes)
			perf.AvgResolutionTime = &avg
			p95 := s.percentileDuration(resolutionTimes, 95)
			perf.P95ResolutionTime = &p95
		}

		if len(tickets) > 0 {
			perf.OverdueRate = float64(overdueCount) / float64(len(tickets))
		}

		// Compute health status
		perf.HealthStatus = s.computeMetricHealth(perf.OverdueRate, perf.AvgResolutionTime, 1*time.Hour)

		adminMetrics = append(adminMetrics, perf)
	}

	return adminMetrics, nil
}

// computeTrends calculates trend comparison between last 24h and previous 24h.
func (s *SLAService) computeTrends(ctx context.Context, tx db.Tx) (*TrendComparison, error) {
	now := time.Now()
	last24hStart := now.Add(-24 * time.Hour)
	previous24hStart := now.Add(-48 * time.Hour)
	previous24hEnd := last24hStart

	// Get last 24h metrics
	supportLast24h, err := s.computeSupportMetrics(ctx, tx, last24hStart, nil)
	if err != nil {
		return nil, err
	}

	// Get previous 24h metrics
	supportPrev24h, err := s.computeSupportMetrics(ctx, tx, previous24hStart, &previous24hEnd)
	if err != nil {
		return nil, err
	}

	// Calculate changes
	responseTimeChange := s.calculatePercentageChange(
		supportLast24h.AvgFirstResponseTime,
		supportPrev24h.AvgFirstResponseTime,
	)
	resolutionTimeChange := s.calculatePercentageChange(
		supportLast24h.AvgResolutionTime,
		supportPrev24h.AvgResolutionTime,
	)
	overdueRateChange := (supportLast24h.OverdueRate - supportPrev24h.OverdueRate) * 100

	return &TrendComparison{
		Last24Hours:          supportLast24h,
		Previous24Hours:      supportPrev24h,
		ResponseTimeChange:   responseTimeChange,
		ResolutionTimeChange: resolutionTimeChange,
		OverdueRateChange:    overdueRateChange,
	}, nil
}

// computeSystemHealth calculates overall system health.
func (s *SLAService) computeSystemHealth(support, dispute *SLAMetricBreakdown, admins []AdminPerformanceMetrics) SystemHealthStatus {
	score := 100.0
	issues := []string{}

	// Check support health
	if support != nil {
		if support.OverdueRate > 0.25 {
			score -= 30
			issues = append(issues, "High support overdue rate")
		} else if support.OverdueRate > 0.10 {
			score -= 15
			issues = append(issues, "Elevated support overdue rate")
		}

		if support.ActiveCount > 50 {
			score -= 20
			issues = append(issues, "High active support ticket count")
		}
	}

	// Check dispute health
	if dispute != nil {
		if dispute.OverdueRate > 0.20 {
			score -= 30
			issues = append(issues, "High dispute overdue rate")
		} else if dispute.OverdueRate > 0.10 {
			score -= 15
			issues = append(issues, "Elevated dispute overdue rate")
		}

		if dispute.ActiveCount > 20 {
			score -= 20
			issues = append(issues, "High active dispute count")
		}
	}

	// Check admin performance
	for _, admin := range admins {
		if admin.OverdueRate > 0.30 {
			score -= 10
			issues = append(issues, "Some admins with high overdue rates")
			break
		}
	}

	// Determine status
	status := "good"
	if score < 50 {
		status = "critical"
	} else if score < 80 {
		status = "warning"
	}

	if len(issues) == 0 {
		issues = []string{"All systems operating normally"}
	}

	return SystemHealthStatus{
		Status: status,
		Score:  math.Max(0, score),
		Issues: issues,
	}
}

// Helper functions

func (s *SLAService) averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func (s *SLAService) percentileDuration(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Sort durations
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate percentile index
	index := int(math.Ceil(float64(len(sorted)) * percentile / 100)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func (s *SLAService) calculatePercentageChange(current, previous *time.Duration) float64 {
	if current == nil || previous == nil || *previous == 0 {
		return 0
	}
	change := float64(*current-*previous) / float64(*previous) * 100
	return math.Round(change*100) / 100
}

func (s *SLAService) computeMetricHealth(overdueRate float64, avgResolution *time.Duration, slaTarget time.Duration) string {
	if overdueRate > 0.25 {
		return "critical"
	}
	if overdueRate > 0.10 {
		return "warning"
	}
	if avgResolution != nil && *avgResolution > slaTarget*2 {
		return "warning"
	}
	return "good"
}


